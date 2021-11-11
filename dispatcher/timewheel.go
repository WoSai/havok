package dispatcher

import (
	"errors"
	"github.com/wosai/havok/internal/logger"
	"sync/atomic"
	"time"

	pb "github.com/wosai/havok/protobuf"
	"go.uber.org/zap"
)

var (
	defaultTimeWheelInterval = 10 * time.Millisecond
	defaultInboxBuffer       = 1000
)

type (
	// timeWheel 用于控制LogRecord发送速率
	TimeWheel struct {
		delta    time.Duration          // 日志时间与当前时间的偏差
		nextStop time.Time              // 下一次等待时间（现实时间）
		offset   time.Duration          // 在增速回放时，原始日志时间戳的修正量
		interval time.Duration          // 每次循环间隔，如1s
		inbox    chan *LogRecordWrapper // 接收LogRecordWrapper，输入方为Fetcher
		status   TaskStatus             // TimeWheel工作状态
		begin    time.Time
		end      time.Time
		speed    float32
		ticker   *time.Ticker
		Havok    *Havok
		parent   ParentTask
		counter  int64
		qps      int64
	}
)

// Stop 停止TimeWheel的分发
func (tw *TimeWheel) Stop() {
	atomic.CompareAndSwapInt32(&tw.status, StatusRunning, StatusStopped)
	if tw.parent != nil { //有ParentTask存在时，不直接通知下游havok
		tw.parent.Notify(tw, StatusStopped)
	} else if tw.Havok != nil {
		tw.Havok.Broadcast(&pb.DispatcherEvent{Type: pb.DispatcherEvent_JobStop})
	}
}

func (tw *TimeWheel) Finish() {
	atomic.CompareAndSwapInt32(&tw.status, StatusRunning, StatusFinished)
	if tw.parent != nil {
		tw.parent.Notify(tw, StatusFinished)
	} else if tw.Havok != nil {
		tw.Havok.Broadcast(&pb.DispatcherEvent{Type: pb.DispatcherEvent_JobStop})
	}
	if tw.ticker != nil {
		tw.ticker.Stop()
	}
}

func (tw *TimeWheel) Status() TaskStatus {
	return atomic.LoadInt32(&tw.status)
}

// NewTimeWheel 构造函数
func NewTimeWheel(job *pb.JobConfiguration) (*TimeWheel, error) {
	if err := checkConfiguration(job); err != nil {
		return nil, err
	}
	return &TimeWheel{
		interval: defaultTimeWheelInterval,
		inbox:    make(chan *LogRecordWrapper, defaultInboxBuffer),
		begin:    ParseMSec(job.Begin),
		end:      ParseMSec(job.End),
		speed:    job.Speed,
	}, nil
}

// Recv 接收待发送的日志，交由TimeWheel对象来按时间顺序投递
func (tw *TimeWheel) Recv() chan<- *LogRecordWrapper {
	return tw.inbox
}

// Start TimeWheel主函数
func (tw *TimeWheel) Start() error {
	atomic.StoreInt32(&tw.status, StatusRunning)
	if tw.parent != nil {
		tw.parent.Notify(tw, StatusRunning)
	}
	logger.Logger.Info("start time wheel", zap.String("begin", tw.begin.String()), zap.String("end", tw.end.String()))

	go func() {
		var last = tw.counter
		var current int64
		for {
			time.Sleep(1 * time.Second)
			current = atomic.LoadInt64(&tw.counter)
			tw.qps = current - last
			last = current
			logger.Logger.Info("TimeWheel QPS", zap.Int64("timewheel_qps", tw.qps))
		}
	}()

	for log := range tw.inbox {
		if atomic.LoadInt32(&tw.status) == StatusStopped {
			logger.Logger.Warn("current time wheel is stopped")
			return ErrTaskInterrupted
		}

		if log.OccurAt.Before(tw.begin) { // 早于任务开始时间，不处理
			continue
		}

		if log.OccurAt.After(tw.end) { // 晚于任务结束时间，不处理，并且结束
			tw.Finish()
			logger.Logger.Info("time of log record is later than end time of job, time wheel would be shutdown", zap.String("occurAt", log.OccurAt.String()))
			return nil
		}

		if tw.delta == 0 {
			tw.delta = time.Now().Sub(log.OccurAt)
			logger.Logger.Info("set delta field of time wheel", zap.String("delta", tw.delta.String()), zap.String("occurAt", log.OccurAt.String()))
			go tw.wheeling()
		}

		//Logger.Info("received log", zap.String("occurAt", log.OccurAt.String()))
		for tw.nextStop.Before(log.OccurAt.Add(tw.delta)) {
			time.Sleep(time.Millisecond)
			//tw.next()
		}

		atomic.AddInt64(&tw.counter, 1)
		tw.Havok.Send(log)
	}
	// 上游Fetcher需要主动关闭channel，应当视为其完成了发送
	logger.Logger.Info("time wheel inbox is closed, fetcher is finished")
	tw.Finish()
	return nil
}

// Parent 关联任务
func (tw *TimeWheel) Parent(p ParentTask) {
	tw.parent = p
}

func (tw *TimeWheel) wheeling() {
	tw.ticker = time.NewTicker(defaultTimeWheelInterval)
	tw.next()
	for range tw.ticker.C {
		tw.next()
	}
	logger.Logger.Info("stop wheeling")
}

func (tw *TimeWheel) next() {
	tw.offset += time.Duration(float64(tw.interval) * float64(tw.speed-1.0))
	tw.nextStop = time.Now().Add(tw.offset + time.Duration(float32(tw.interval)*tw.speed))
}

// WithHavok 设置接收LogRecord的havok服务
func (tw *TimeWheel) WithHavok(hv *Havok) *TimeWheel {
	tw.Havok = hv
	return tw
}

func (tw *TimeWheel) refreshConfig(c *pb.JobConfiguration) error {
	if c == nil || c.Begin == 0 || c.End == 0 || c.Speed <= 0 {
		return errors.New("bad job Configuration")
	}
	tw.begin = ParseMSec(c.Begin)
	tw.end = ParseMSec(c.End)
	tw.speed = c.Speed
	return nil
}
