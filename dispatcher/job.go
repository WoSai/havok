package dispatcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/wosai/havok/pkg/genproto"
	"go.uber.org/zap"
)

type (
	// Job 任务对象
	Job struct {
		Configuration   *pb.JobConfiguration
		fetcher         Fetcher
		timeWheel       *TimeWheel
		Havok           *Havok // 不作为子任务，因此不允许子任务直接操作havok
		status          TaskStatus
		fetcherStatus   TaskStatus
		timeWheelStatus TaskStatus
		feature         *Feature
		lock            sync.Mutex
	}

	Feature struct {
		Shake  *config
		Strike *config
	}

	config struct {
		Peak        float32 `json:"peak,omitempty"`
		Interval    int32   `json:"interval,omitempty"`
		Coverage    int32   `json:"coverage,omitempty"`
		Probability float32 `json:"probability,omitempty"`
	}
)

const defaultContentType = "application/json"

func checkConfiguration(c *pb.JobConfiguration) error {
	if c == nil || ParseMSec(c.Begin).IsZero() || ParseMSec(c.End).IsZero() || c.Rate <= 0 || c.Speed <= 0 || c.Stuck < 0 {
		return errors.New("bad job configuration value")
	}
	return nil
}

// NewJob Job的构造函数
func NewJob(c *pb.JobConfiguration) (*Job, error) {
	if err := checkConfiguration(c); err != nil {
		return nil, err
	}
	return &Job{Configuration: c, status: StatusReady, feature: &Feature{Shake: &config{}, Strike: &config{}}}, nil
}

func (job *Job) mergeJobConfiguration(d interface{}) {
	job.lock.Lock()
	defer job.lock.Unlock()
	switch d.(type) {
	case *pb.JobConfiguration:
		target := d.(*pb.JobConfiguration)
		mergeJob(job.Configuration, target)
	case *Feature:
		target := d.(*Feature)
		if target.Shake != nil {
			mergeFeatureConfig(job.feature.Shake, target.Shake)
		}
		if target.Strike != nil {
			mergeFeatureConfig(job.feature.Strike, target.Strike)
		}
	}
}

func mergeJob(c1, c2 *pb.JobConfiguration) {
	if c2.Begin > 0 && c2.Begin != c1.Begin {
		c1.Begin = c2.Begin
	}
	if c2.End > 0 && c2.End != c1.End {
		c1.End = c2.End
	}
	if c2.Rate > 0 && c2.Rate != c1.Rate {
		c1.Rate = c2.Rate
	}
	if c2.Speed > 0 && c2.Speed != c1.Speed {
		c1.Speed = c2.Speed
	}
	if c2.Stuck >= 0 && c2.Stuck != c1.Stuck {
		c1.Stuck = c2.Stuck
	}
}

func mergeFeatureConfig(c1, c2 *config) {
	if c2.Peak > 0 && c2.Peak != c1.Peak {
		c1.Peak = c2.Peak
	}
	if c2.Interval > 0 && c2.Interval != c1.Interval {
		c1.Interval = c2.Interval
	}
	if c2.Coverage > 0 && c2.Coverage != c1.Coverage {
		c1.Coverage = c2.Coverage
	}
	if c2.Probability >= 0 && c2.Probability != c1.Probability {
		c1.Probability = c2.Probability
	}
}

func (job *Job) Provide() []ProviderMethod {
	return []ProviderMethod{
		{
			Path: "/api/job/start",
			Func: func(writer http.ResponseWriter, request *http.Request) {
				if job.Status() == StatusReady {
					c := new(pb.JobConfiguration)
					err := json.NewDecoder(request.Body).Decode(c)
					if err != nil {
						renderError(writer, err)
						return
					}
					job.mergeJobConfiguration(c)
					job.timeWheel.begin = ParseMSec(job.Configuration.Begin)
					job.timeWheel.end = ParseMSec(job.Configuration.End)
					job.timeWheel.speed = job.Configuration.Speed
					go job.Start()
					renderResponse(writer, []byte(`{"code": 200, "msg": "job started"}`), defaultContentType)
					return
				}
				renderError(writer, errors.New("bad job status"))
			},
		},
		{
			Path: "/api/job/description",
			Func: func(writer http.ResponseWriter, request *http.Request) {
				resp := &struct {
					Code    int                   `json:"code"`
					Data    map[string]TaskStatus `json:"data"`
					Feature *Feature              `json:"feature"`
				}{Code: http.StatusOK, Data: job.Description(), Feature: job.feature}
				body, err := json.Marshal(resp)
				if err != nil {
					renderError(writer, err)
					return
				}
				renderResponse(writer, body, "application/json")
			},
		},
		{
			Path: "/api/job/shake",
			Func: func(writer http.ResponseWriter, request *http.Request) {
				c := new(config)
				err := json.NewDecoder(request.Body).Decode(c)
				if err != nil {
					renderError(writer, err)
					return
				}
				job.mergeJobConfiguration(&Feature{Shake: c})
				renderResponse(writer, []byte(`{"code": 200, "msg": "succeeded to refresh shake configuration"}`), defaultContentType)
			},
		},
		{
			Path: "/api/job/strike",
			Func: func(writer http.ResponseWriter, request *http.Request) {
				c := new(config)
				err := json.NewDecoder(request.Body).Decode(c)
				if err != nil {
					renderError(writer, err)
					return
				}
				job.mergeJobConfiguration(&Feature{Strike: c})
				renderResponse(writer, []byte(`{"code": 200, "msg": "succeeded to refresh strike configuration"}`), defaultContentType)
			},
		},
	}
}

// Start 任务开始
func (job *Job) Start() error {
	atomic.StoreInt32(&job.status, StatusRunning)

	job.Havok.Broadcast(&pb.DispatcherEvent{
		Type: pb.DispatcherEvent_JobStart,
		Data: &pb.DispatcherEvent_Job{job.Configuration}},
	)

	job.timeWheel.WithHavok(job.Havok)
	go job.timeWheel.Start()
	job.fetcher.TimeRange(ParseMSec(job.Configuration.Begin), ParseMSec(job.Configuration.End))
	job.fetcher.SetOutput(job.timeWheel.Recv())
	go job.fetcher.Start()
	go job.featureShake()
	go job.featureStrike()
	return nil
}

// Stop 任务停止
func (job *Job) Stop() {
	atomic.CompareAndSwapInt32(&job.status, StatusRunning, StatusStopped)
	job.Havok.Broadcast(&pb.DispatcherEvent{Type: pb.DispatcherEvent_JobStop})
}

// Finish 任务完成
func (job *Job) Finish() {
	atomic.CompareAndSwapInt32(&job.status, StatusRunning, StatusFinished)
	job.Havok.Broadcast(&pb.DispatcherEvent{Type: pb.DispatcherEvent_JobFinish})
}

// Broadcast 只接收子任务通知，不负责更新下游子任务状态
func (job *Job) Notify(from SubTask, status TaskStatus) {
	Logger.Info("received notification from sub task", zap.Int32("status", status))
	switch status {
	case StatusStopped:
		switch from.(type) {
		case Fetcher:
			Logger.Info("fetcher has be stopped")
			atomic.StoreInt32(&job.fetcherStatus, StatusStopped)
		case *TimeWheel:
			Logger.Info("time wheel has be stopped")
			atomic.StoreInt32(&job.timeWheelStatus, StatusStopped)
			job.Stop()
		default:
		}
	case StatusFinished:
		switch from.(type) {
		case Fetcher:
			Logger.Info("fetcher has be finished")
			atomic.StoreInt32(&job.fetcherStatus, StatusFinished)
		case *TimeWheel:
			Logger.Info("time wheel has be finished")
			atomic.StoreInt32(&job.timeWheelStatus, StatusFinished)
			job.Finish()
		default:
		}
	case StatusRunning:
		switch from.(type) {
		case Fetcher:
			atomic.StoreInt32(&job.fetcherStatus, StatusRunning)
		case *TimeWheel:
			atomic.StoreInt32(&job.timeWheelStatus, StatusRunning)
		}
	}
}

func (job *Job) Description() map[string]TaskStatus {
	return map[string]TaskStatus{"Fetcher": job.fetcherStatus, "TimeWheel": job.timeWheelStatus}
}

// Status 获取任务状态
func (job *Job) Status() TaskStatus {
	return atomic.LoadInt32(&job.status)
}

// WithFetcher 设置任务使用Fetcher
func (job *Job) WithFetcher(f Fetcher) *Job {
	job.fetcher = f
	job.fetcher.Parent(job)
	return job
}

// WithTimeWheel 设置控制日志发送速率的TimeWheel
func (job *Job) WithTimeWheel(tw *TimeWheel) *Job {
	job.timeWheel = tw
	job.timeWheelStatus = tw.status
	tw.refreshConfig(job.Configuration)
	tw.Parent(job)
	return job
}

// UseDefaultHavok 使用内置的havok服务
func (job *Job) UseDefaultHavok() *Job {
	return job.WithHavok(DefaultHavok)
}

// WithHavok 设置任务使用的havok服务
func (job *Job) WithHavok(hv *Havok) *Job {
	job.Havok = hv
	return job
}

func (job *Job) featureShake() {
	//shake模拟流量锯齿特性
	var peak, probability float32
	var interval int32
	for {
		peak = job.feature.Shake.Peak
		interval = job.feature.Shake.Interval
		probability = job.feature.Shake.Probability
		if probability > 0 {
			if r := rand.Float32(); probability >= r {
				ss := int64(rand.Float32() * peak * 1000)
				job.mergeJobConfiguration(&pb.JobConfiguration{Rate: -1, Stuck: ss})
				job.Havok.Broadcast(&pb.DispatcherEvent{
					Type: pb.DispatcherEvent_JobConfiguration,
					Data: &pb.DispatcherEvent_Job{job.Configuration},
				})
				Logger.Info("trigger refresh shake feature config", zap.Any("shake feature config", job.Configuration))
			}
			time.Sleep(time.Second * time.Duration(interval))
		} else {
			//任务没有必要跑那么快
			time.Sleep(time.Millisecond * 200)
		}
	}
}

func (job *Job) featureStrike() {
	// strike模拟异常流量特性
	var orgRate, peak, probability float32
	var interval, coverage int32
	for {
		peak = job.feature.Strike.Peak
		interval = job.feature.Strike.Interval
		coverage = job.feature.Strike.Coverage
		probability = job.feature.Strike.Probability
		if probability > 0 {
			if r := rand.Float32(); probability >= r {
				rateFormat, err := strconv.ParseFloat(fmt.Sprintf("%.2f", rand.Float32()*peak), 32)
				if err == nil {
					rate := float32(rateFormat)
					orgRate = job.Configuration.Rate
					job.mergeJobConfiguration(&pb.JobConfiguration{Rate: rate, Stuck: -1})
					job.Havok.Broadcast(&pb.DispatcherEvent{
						Type: pb.DispatcherEvent_JobConfiguration,
						Data: &pb.DispatcherEvent_Job{job.Configuration},
					})
					Logger.Info("trigger refresh strike feature config", zap.Any("strike feature config", job.Configuration))
					time.Sleep(time.Second * time.Duration(coverage))
					job.mergeJobConfiguration(&pb.JobConfiguration{Rate: orgRate, Stuck: -1})
					job.Havok.Broadcast(&pb.DispatcherEvent{
						Type: pb.DispatcherEvent_JobConfiguration,
						Data: &pb.DispatcherEvent_Job{job.Configuration},
					})
					Logger.Info("trigger recover strike feature config", zap.Any("strike feature config", job.Configuration))
				}
			}
			time.Sleep(time.Second * time.Duration(interval))
		} else {
			//任务没有必要跑那么快
			time.Sleep(time.Millisecond * 200)
		}
	}
}
