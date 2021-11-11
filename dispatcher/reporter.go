package dispatcher

import (
	"github.com/wosai/havok/internal/logger"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/wosai/havok/protobuf"
	"github.com/wosai/havok/types"
	"go.uber.org/zap"
)

type (
	Reporter struct {
		rm                 *ReplayerManager
		ReportHandler      []ReportHandleFunc
		CollectInterval    time.Duration
		timeout            time.Duration
		batch              int32 // 当前批次
		reservoirs         map[int32]*reservoir
		lastCompletedBatch int32 // 最后完成的批次
		signal             chan int32
		lastReport         types.Report
		mu                 sync.RWMutex
	}

	ReportHandleFunc func(types.Report, types.PerformanceStat)

	// reservoir 等待replayer回调SummaryStats
	reservoir struct {
		summary  *types.SummaryStats
		expected int32
		replied  int32
		since    time.Time
		timeout  time.Duration
		perfStat types.PerformanceStat
	}

	Provider interface {
		Provide() []ProviderMethod
	}

	ProviderMethod struct {
		Path string
		Func http.HandlerFunc
	}
)

func NewReporter(rm *ReplayerManager, h ...ReportHandleFunc) *Reporter {
	return &Reporter{
		rm:                 rm,
		ReportHandler:      h,
		CollectInterval:    5 * time.Second,
		timeout:            3 * time.Second,
		reservoirs:         map[int32]*reservoir{},
		lastCompletedBatch: -1,
		signal:             make(chan int32, 1),
	}
}

func (r *Reporter) WithReplayerManager(rm *ReplayerManager) {
	if rm != nil {
		r.rm = rm
	}
}

func (r *Reporter) PeriodicRequest() {
	for {
		time.Sleep(r.CollectInterval)

		batch := atomic.AddInt32(&r.batch, 1)
		rs := r.rm.GetReplayers()
		nums := len(rs)
		if nums == 0 {
			logger.Logger.Warn("no alived replayer, no report request sent")
			continue
		}

		de := &pb.DispatcherEvent{
			Type: pb.DispatcherEvent_StatsCollection,
			Data: &pb.DispatcherEvent_Stats{Stats: &pb.StatsRequest{
				RequestId:   batch,
				RequestTime: time.Now().UnixNano() / 1e6,
			}}}

		reservoir := newReservoir(int32(nums), r.timeout)
		r.mu.Lock()
		r.reservoirs[batch] = reservoir //  先占位，再发，要不然replayer太快受不住
		r.mu.Unlock()
		for _, rep := range rs {
			err := r.rm.Deliver(rep.ID, de)
			if err != nil {
				atomic.AddInt32(&reservoir.expected, -1)
				nums--
			}
		}

		if nums > 0 {
			logger.Logger.Info("start to collect stats from all alived replayers", zap.Int32("batch", batch), zap.Int("alived-replayer", nums))
		}
	}
}

func (r *Reporter) Collect(rid string, b int32, perStat map[string]float64, stats ...*pb.AttackerStatsWrapper) {
	logger.Logger.Info("collected report", zap.String("replayer", rid), zap.Int32("batch", b))
	r.mu.Lock()
	defer r.mu.Unlock()

	res, exist := r.reservoirs[b]
	if !exist {
		logger.Logger.Warn("cannot found reservoir", zap.Int32("batch-to-collect", b), zap.Int32("last-complete-batch", r.lastCompletedBatch), zap.Int32("latest-batch", r.batch))
		return
	}
	res.convertAndAggregate(rid, stats...)
	res.perfStat.Stats[rid] = perStat
	if res.isReady() {
		r.signal <- b
	}
}

func (r *Reporter) Run() {
	for {
		batch := <-r.signal

		r.mu.Lock()
		last := r.lastCompletedBatch

		if batch < last {
			logger.Logger.Warn("bad batch number, it will be delete", zap.Int32("batch", batch))
			r.mu.Unlock()
			continue
		}

		res := r.reservoirs[batch]
		r.lastReport = res.summary.Report(false)
		if !res.summary.IsZero() {
			go func(report types.Report, perfStat types.PerformanceStat) {
				for _, h := range r.ReportHandler {
					h(report, perfStat)
				}
			}(r.lastReport, res.perfStat)
		} else {
			logger.Logger.Warn("SummaryStats is empty, ignored", zap.Int32("batch", batch))
		}

		for ; last <= batch; last++ {
			if last >= 0 { // 初始值为-1
				delete(r.reservoirs, last)
			}
		}
		r.lastCompletedBatch = batch
		r.mu.Unlock()
	}
}

func (r *Reporter) Provide() []ProviderMethod {
	return []ProviderMethod{
		{
			Path: "/api/reporter/last_report",
			Func: func(writer http.ResponseWriter, request *http.Request) {
				r.mu.RLock()
				defer r.mu.RUnlock()
				renderJSON(writer, map[string]interface{}{"batch": r.lastCompletedBatch, "report": r.lastReport})
			},
		},
	}
}

func newReservoir(expected int32, timeout time.Duration) *reservoir {
	return &reservoir{
		summary:  types.NewSummaryStats(),
		expected: expected,
		since:    time.Now(),
		timeout:  timeout,
		perfStat: types.PerformanceStat{Stats: make(map[string]map[string]float64)},
	}
}

func (rc *reservoir) convertAndAggregate(rid string, stats ...*pb.AttackerStatsWrapper) {
	atomic.AddInt32(&rc.replied, 1)
	ss := types.NewSummaryStats()
	for _, s := range stats {
		ss.Store(types.Unwrap(s))
		//Logger.Info("stats", zap.Any(s.Name, s), zap.String("replayer", rid))
	}
	rc.summary.Aggregate(ss)
}

func (rc *reservoir) isReady() bool {
	return atomic.LoadInt32(&rc.replied) == rc.expected
}

func (rc *reservoir) hasTimeout() bool {
	return time.Since(rc.since) >= rc.timeout
}
