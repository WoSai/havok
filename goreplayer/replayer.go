package replayer

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/wosai/havok/internal/logger"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	dispatcher "github.com/wosai/havok/protobuf"
	"github.com/wosai/havok/types"
	"go.uber.org/zap"
	"reflect"
)

type (
	Replayer struct {
		replayRate  float32
		PH          *ProcessorHub
		Selector    APISelector
		Concurrency int
		ch          chan struct{}
		client      *http.Client
		stuck       time.Duration
		lock        sync.RWMutex
	}

	APISelector func(url *url.URL, header map[string]string, method string, body []byte) HTTPAPI

	Packager struct {
		Session *types.Session
		Header  http.Header
		URL     *url.URL
		Body    []byte
	}
)

func DefaultSelector(_ *url.URL, _ map[string]string, _ string, _ []byte) HTTPAPI {
	return "default"
}

func defaultHTTPClient(keepAlive bool) *http.Client {
	disableKeepAlive := !keepAlive
	return &http.Client{
		Timeout: 90 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			DisableKeepAlives:     disableKeepAlive,
			MaxIdleConns:          2000,
			MaxIdleConnsPerHost:   1000,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func NewReplayer(rate float32, pc *ProcessorHub, c int, keepAlive bool) *Replayer {
	rep := &Replayer{
		replayRate:  rate,
		PH:          pc,
		Concurrency: c,
		client:      defaultHTTPClient(keepAlive),
	}
	rep.ch = make(chan struct{}, c)
	return rep
}

func (rep *Replayer) Run() {
	for logRecord := range replayerPipeline {
		rate := rep.replayRate
		var api HTTPAPI = "default"
		reqURL, err := url.Parse(logRecord.Url)
		if err != nil {
			logger.Logger.Error("failed to parse url", zap.Error(err))
			continue
		}

		// 模拟锯齿峰请求特性
		if hold := rep.getStuck(); hold > 0 {
			time.Sleep(time.Millisecond * hold)
		}

		if rep.Selector != nil {
			api = rep.Selector(reqURL, logRecord.Header, logRecord.Method, logRecord.Body)
		}

		for rate > 0 {
			if rate < 1 {
				if r := rand.Float32(); r >= rate {
					break
				}
			}
			rate--

			rep.ch <- struct{}{}
			go func(u *url.URL, record *dispatcher.LogRecord, httpAPI HTTPAPI) {
				defer func() { <-rep.ch }()
				duration, err := rep.send(httpAPI, u, record.Method, record.Header, record.Body, nil)
				resultPipeline <- types.NewResult(string(httpAPI), duration, err)
			}(reqURL, logRecord, api)
		}
	}
}

func (rep *Replayer) Register(selector APISelector) {
	rep.Selector = selector
}

func (rep *Replayer) refreshReplayerConfig(jobConfig *dispatcher.JobConfiguration) {
	rep.lock.Lock()
	defer rep.lock.Unlock()
	if jobConfig != nil {
		if rep.replayRate != jobConfig.Rate && jobConfig.Rate > 0 {
			rep.replayRate = jobConfig.Rate
			logger.Logger.Info("refresh replayer rate", zap.Float32("rate", jobConfig.Rate))
		}
		stuck := time.Duration(jobConfig.Stuck)
		if rep.stuck != stuck {
			rep.stuck = stuck
			logger.Logger.Info("refresh replayer stuck", zap.Int64("stuck millisecond", jobConfig.Stuck))
		}
	}

}

// 保证一定时间内stuck只会出现一次>0的情况
func (rep *Replayer) getStuck() time.Duration {
	rep.lock.Lock()
	defer rep.lock.Unlock()
	tempStuck := rep.stuck
	if tempStuck > 0 {
		rep.stuck = types.ZeroDuration
		logger.Logger.Info("trigger stuck", zap.Duration("value", tempStuck))
		return tempStuck
	}
	return types.ZeroDuration
}

//手动进行extra send操作, data存储格式为[]map[string][]dispatcher.LogRecord{}, 并且共享一份session(即下面所有的send操作均共享同一份session)
//其中string为action的name，即同一批次的action下所有的[]dispatcher.LogRecord{}均会同时异步调用send，不同批次的action之间会阻塞执行，可以涵盖大部分场景
func (rep *Replayer) ExtraSendWithSession(data interface{}, session *types.Session) {
	if extras, ok := data.([]map[string][]interface{}); ok {
		for _, extra := range extras {
			var wg sync.WaitGroup
			for name, logs := range extra {
				for _, log := range logs {
					switch logRecord := log.(type) {
					case dispatcher.LogRecord:
						logger.Logger.Debug("try to do extra send", zap.String("ActionName", name), zap.Any("LogRecord", logRecord))
						reqURL, err := url.Parse(logRecord.Url)
						if err != nil {
							logger.Logger.Error("ExtraSend failed to parse url", zap.Error(err))
						} else {
							httpAPI := rep.Selector(reqURL, logRecord.Header, logRecord.Method, logRecord.Body)
							wg.Add(1)
							go func(u *url.URL, record *dispatcher.LogRecord, httpAPI HTTPAPI, s *types.Session) {
								defer wg.Done()
								duration, err := rep.send(httpAPI, u, record.Method, record.Header, record.Body, s)
								resultPipeline <- types.NewResult(string(httpAPI), duration, err)
							}(reqURL, &logRecord, httpAPI, session)
						}
					default:
						logger.Logger.Info("unknown extra action", zap.Any("type", reflect.TypeOf(log)), zap.String("ActionName", name), zap.Any("LogRecord", logRecord))
					}
				}
			}
			wg.Wait()
		}
	}
}

func (rep *Replayer) send(api HTTPAPI, oU *url.URL, method string, oH map[string]string, oB []byte, s *types.Session) (time.Duration, error) {
	if s == nil {
		s = sessionPool.Get()
		defer sessionPool.Put(s)
	}

	u := copyUrl(oU)
	h := convertHeader(oH)
	var b []byte
	if oB != nil {
		b = make([]byte, len(oB))
		copy(b, oB)
	}

	var scope FlowScope = "request"
	var duration time.Duration = 0
	var err error

	// 进行前置操作，修改request信息
	pb := rep.PH.GetProcessor(api, scope)
	requestPackager := &Packager{Session: s, Header: h, URL: u, Body: b}
	requestPackager, err = replayerBeforeAction(pb, requestPackager)
	if err != nil {
		return duration, err
	}

	// doRequest
	req, err := http.NewRequest(method, requestPackager.URL.String(), bytes.NewBuffer(requestPackager.Body))
	if err != nil {
		logger.Logger.Error("failed to new request", zap.Error(err))
		return 0, err
	}
	req.Header = requestPackager.Header
	req.Header["fake"] = []string{"1"}

	logger.Logger.Info("prepare to request", zap.String("url", requestPackager.URL.String()), zap.Any("req header", req.Header), zap.Any("cookies", req.Cookies()), zap.ByteString("body", requestPackager.Body))
	start := time.Now()
	res, err := rep.client.Do(req)
	duration = time.Since(start)
	if err != nil {
		logger.Logger.Error("occur error when send request", zap.Error(err), zap.ByteString("req body", requestPackager.Body), zap.String("url", req.URL.String()))
		return duration, err
	}
	defer res.Body.Close()
	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logger.Logger.Error("occur error when reading response", zap.Error(err), zap.ByteString("req body", requestPackager.Body), zap.String("url", req.URL.String()))
		return duration, err
	}
	duration = time.Since(start)
	logger.Logger.Info("get callback", zap.String("url", req.URL.String()), zap.ByteString("response", content), zap.Int("latency", int(duration.Seconds()*1000)), zap.Any("resp header", res.Header), zap.ByteString("req body", requestPackager.Body))

	if res.StatusCode >= 400 {
		logger.Logger.Error(fmt.Sprintf("bad status code: %d", res.StatusCode), zap.ByteString("req body", requestPackager.Body), zap.String("url", req.URL.String()))
		return duration, errors.New(fmt.Sprintf("bad status code: %d", res.StatusCode))
	}

	scope = "response"
	// 进行后置操作
	pb = rep.PH.GetProcessor(api, scope)
	// 后置assert操作
	responsePackager := &Packager{Session: s, Header: res.Header, URL: res.Request.URL, Body: content}
	if err = replayerAssert(pb, responsePackager); err != nil {
		return duration, err
	}
	// 后置action操作
	if err = replayerAfterAction(pb, responsePackager); err != nil {
		return duration, err
	}

	//doExtra
	if pb != nil && pb.extra != nil {
		if data, err := pb.extra.Act(s, responsePackager); err == nil {
			es := sessionPool.Get()
			es.Copy(s)
			go func(d interface{}, session *types.Session) {
				defer sessionPool.Put(session)
				rep.ExtraSendWithSession(data, session)
			}(data, es)
		} else {
			return duration, err
		}
	}
	return duration, nil
}

func replayerBeforeAction(pb *processorBlock, packager *Packager) (*Packager, error) {
	if pb == nil {
		return packager, nil
	}
	if pb.header != nil {
		logger.Logger.Debug("header before process", zap.Any("header", packager.Header))
		if _, err := pb.header.Act(packager.Session, packager.Header); err == nil {
			logger.Logger.Debug("header after process", zap.Any("header", packager.Header))
		} else {
			return nil, err
		}
	}
	if pb.url != nil {
		logger.Logger.Debug("url before process", zap.Any("url", packager.URL))
		if _, err := pb.url.Act(packager.Session, packager.URL); err == nil {
			logger.Logger.Debug("url after process", zap.Any("url", packager.URL))
		} else {
			return nil, err
		}
	}
	if pb.params != nil {
		logger.Logger.Debug("url params before process", zap.Any("params", packager.URL.Query()))
		if qi, err := pb.params.Act(packager.Session, packager.URL.Query()); err == nil {
			packager.URL.RawQuery = qi.(url.Values).Encode()
			logger.Logger.Debug("url params after process", zap.Any("params", packager.URL.Query()))
		} else {
			return nil, err
		}
	}
	if pb.body != nil {
		logger.Logger.Debug("request body before process", zap.ByteString("body", packager.Body))
		if nb, err := pb.body.Act(packager.Session, packager.Body); err == nil {
			switch t := nb.(type) {
			case url.Values:
				packager.Body = []byte(t.Encode())
			case []byte:
				packager.Body = t
			default:
				return nil, errors.New(fmt.Sprintf("unknown data type: %s", t))
			}
			logger.Logger.Debug("request body after process", zap.ByteString("body", packager.Body))
		} else {
			return nil, err
		}
	}
	return packager, nil
}

func replayerAfterAction(pb *processorBlock, packager *Packager) error {
	if pb == nil {
		return nil
	}
	if pb.header != nil {
		if _, err := pb.header.Act(packager.Session, packager.Header); err != nil {
			logger.Logger.Debug("after action header occur assert", zap.Error(err))
			return err
		}
	}
	if pb.url != nil {
		if _, err := pb.url.Act(packager.Session, packager.Header); err != nil {
			logger.Logger.Debug("after action url occur assert", zap.Error(err))
			return err
		}
	}
	if pb.params != nil {
		if _, err := pb.params.Act(packager.Session, packager.URL.Query()); err != nil {
			logger.Logger.Debug("after action url params occur assert", zap.Error(err))
			return err
		}
	}
	if pb.body != nil {
		if _, err := pb.body.Act(packager.Session, packager.Body); err != nil {
			logger.Logger.Debug("after action body occur assert", zap.Error(err))
			return err
		}
	}
	return nil
}

func replayerAssert(pb *processorBlock, packager *Packager) error {
	if pb == nil {
		return nil
	}
	if pb.header != nil {
		if err := pb.header.Assert(packager.Session, packager.Header); err != nil {
			logger.Logger.Debug("assert header occur assert", zap.Error(err))
			return err
		}
	}
	if pb.url != nil {
		if err := pb.url.Assert(packager.Session, packager.URL); err != nil {
			logger.Logger.Debug("assert url occur assert", zap.Error(err))
			return err
		}
	}
	if pb.params != nil {
		if err := pb.params.Assert(packager.Session, packager.URL.Query()); err != nil {
			logger.Logger.Debug("assert url params occur assert", zap.Error(err))
			return err
		}
	}
	if pb.body != nil {
		if err := pb.body.Assert(packager.Session, packager.Body); err != nil {
			logger.Logger.Debug("assert body occur assert", zap.Error(err))
			return err
		}
	}
	return nil
}

func copyUrl(origin *url.URL) *url.URL {
	n := new(url.URL)
	*n = *origin
	return n
}

func convertHeader(h map[string]string) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		h2.Set(k, vv)
	}
	return h2
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}
