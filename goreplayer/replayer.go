package replayer

import (
	"context"
	"fmt"
	"github.com/wosai/havok/pkg"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	dispatcher "github.com/wosai/havok/protobuf"
	"github.com/wosai/havok/types"
	"go.uber.org/zap"
)

var (
	DefaultReplayer *replayer
)

type (
	replayer struct {
		httpHandler pkg.Handler
		replayRate  float32
		middlewares []pkg.Middleware
		Concurrency int
		ch          chan struct{}
		stuck       time.Duration
		lock        sync.RWMutex
	}
)

func BuildReplayer(c int, httpHandler pkg.Handler, ms []pkg.Middleware) *replayer {
	rep := &replayer{
		httpHandler: httpHandler,
		replayRate:  1.0,
		middlewares: ms,
		Concurrency: c,
	}
	rep.ch = make(chan struct{}, c)
	DefaultReplayer = rep
	return rep
}

func (rep *replayer) Run() {
	var ctx = context.Background()
	for logRecord := range replayerPipeline {
		fmt.Println("get a log")

		rate := rep.replayRate

		// 模拟锯齿峰请求特性
		if hold := rep.getStuck(); hold > 0 {
			time.Sleep(time.Millisecond * hold)
		}

		for rate > 0 {
			if rate < 1 {
				if r := rand.Float32(); r >= rate {
					break
				}
			}
			rate--

			rep.ch <- struct{}{}
			go func(record *dispatcher.LogRecord) {
				defer func() { <-rep.ch }()

				reqURL, err := url.Parse(record.Url)
				if err != nil {
					Logger.Error("failed to parse url", zap.Error(err))
					return
				}

				req := &pkg.Payload{
					Method: record.Method,
					URL:    reqURL,
					Header: convertHeader(record.Header),
					Body:   record.Body,
				}

				t := &timer{}
				mid := append(rep.middlewares, TimerMiddleware(t))
				err = chain(mid, rep.httpHandler).Handle(ctx, req, &httpResponseReader{})

				apiName := req.Name
				if apiName == "" {
					apiName = reqURL.Path
				}
				resultPipeline <- types.NewResult(apiName, t.Duration(), err)
			}(logRecord)
		}
	}
}

func (rep *replayer) refreshReplayerConfig(jobConfig *dispatcher.JobConfiguration) {
	rep.lock.Lock()
	defer rep.lock.Unlock()
	if jobConfig != nil {
		if rep.replayRate != jobConfig.Rate && jobConfig.Rate > 0 {
			rep.replayRate = jobConfig.Rate
			Logger.Info("refresh replayer rate", zap.Float32("rate", jobConfig.Rate))
		}
		stuck := time.Duration(jobConfig.Stuck)
		if rep.stuck != stuck {
			rep.stuck = stuck
			Logger.Info("refresh replayer stuck", zap.Int64("stuck millisecond", jobConfig.Stuck))
		}
	}

}

// 保证一定时间内stuck只会出现一次>0的情况
func (rep *replayer) getStuck() time.Duration {
	rep.lock.Lock()
	defer rep.lock.Unlock()
	tempStuck := rep.stuck
	if tempStuck > 0 {
		rep.stuck = types.ZeroDuration
		Logger.Info("trigger stuck", zap.Duration("value", tempStuck))
		return tempStuck
	}
	return types.ZeroDuration
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
