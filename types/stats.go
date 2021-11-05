package types

import (
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/WoSai/havok/protobuf"
	"go.uber.org/zap"
)

var (
	timeDistributions = [10]float64{0.5, 0.6, 0.7, 0.8, 0.9, 0.95, 0.97, 0.98, 0.99, 1.0}
)

const (
	// ZeroDuration 0，用于一些特殊判断
	ZeroDuration = time.Duration(0)
	// StatsReportInterval 统计报表输出间隔
	StatsReportInterval = 5 * time.Second

	CurrentQPSTimeRange = 12 * time.Second
)

type (
	AttackerStats struct {
		name              string                       // 统计对象名称
		numRequests       int64                        // 成功请求次数
		numFailures       int64                        // 失败次数
		totalResponseTime time.Duration                // 成功请求的response time总和，基于原始的响应时间
		minResponseTime   time.Duration                // 最快的响应时间
		maxResponseTime   time.Duration                // 最慢的响应时间
		trendSuccess      *LimitedSizeMap              // 按时间轴（秒级）记录成功请求次数
		trendFailures     map[int64]int64              // 按时间轴（秒级）记录错误的请求次数
		responseTimes     map[roundedMillisecond]int64 // 按优化后的响应时间记录成功请求次数
		failuresTimes     map[string]int64             // 记录不同错误的次数
		startTime         time.Time                    // 第一次收到请求的时间(包括成功以及失败)
		lastRequestTime   time.Time                    // 最后一次收到请求的时间(包括成功以及失败)
		interval          time.Duration                // 统计时间间隔，影响 CurrentQPS()
		lock              sync.RWMutex
	}

	//AttackerStatsWrapper struct {
	//	Name              string           `json:"name"`
	//	Requests          int64            `json:"requests"`
	//	Failures          int64            `json:"failures"`
	//	TotalResponseTime int64            `json:"total_response_time"` // 毫秒级别
	//	MinResponseTime   int64            `json:"min_response_time"`   // 毫秒级别
	//	MaxResponseTime   int64            `json:"max_response_time"`
	//	TrendSuccess      map[int64]int64  `json:"trend_success"` // key为unix时间戳，秒级
	//	TrendFailures     map[int64]int64  `json:"trend_failures"`
	//	ResponseTimes     map[int64]int64  `json:"response_times"`
	//	FailureTimes      map[string]int64 `json:"failure_times"`
	//	StartTime         int64            `json:"start_time"`        // 毫秒级别
	//	LastRequestTime   int64            `json:"last_request_time"` // 毫秒级别
	//}

	SummaryStats struct {
		nodes sync.Map
	}

	AttackerError struct {
		Name     string
		CausedBy string
	}

	Result struct {
		Name     string
		Duration int64
		Error    *AttackerError
	}

	LimitedSizeMap struct {
		content map[int64]int64
		size    int64
	}

	PerformanceStat struct {
		Stats map[string]map[string]float64
	}

	// AttackerReport Attacker级别的报告
	AttackerReport struct {
		Name           string           `json:"name"`
		Requests       int64            `json:"requests"`
		Failures       int64            `json:"failures"`
		Min            int64            `json:"min"`
		Max            int64            `json:"max"`
		Median         int64            `json:"median"`
		Average        int64            `json:"average"`
		QPS            int64            `json:"qps"`
		Distributions  map[string]int64 `json:"distributions"`
		FailRatio      string           `json:"fail_ratio"`
		FailureDetails map[string]int64 `json:"failure_details"`
		FullHistory    bool             `json:"full_history"`
	}

	// Report Task级别的报告
	Report map[string]*AttackerReport

	roundedMillisecond int64
)

func timeDurationToRoundedMillisecond(t time.Duration) roundedMillisecond {
	ms := int64(t.Seconds()*1000 + 0.5)
	var rm roundedMillisecond
	if ms < 100 {
		rm = roundedMillisecond(ms)
	} else if ms < 1000 {
		rm = roundedMillisecond(ms + 5 - (ms+5)%10)
	} else {
		rm = roundedMillisecond(ms + 50 - (ms+50)%100)
	}
	return rm
}

func roundedMillisecondToDuration(r roundedMillisecond) time.Duration {
	return time.Duration(r * 1000 * 1000)
}

func timeDurationToMillisecond(t time.Duration) int64 {
	return int64(t) / int64(time.Millisecond)
}

func msToNS(ms int64) time.Duration {
	return time.Duration(ms * 1e6)
}

func timeFromTimestamp(ms int64) time.Time {
	sec := ms / 1e3
	return time.Unix(sec, (ms-sec*1e3)*1e6)
}

func Unwrap(asw *pb.AttackerStatsWrapper) *AttackerStats {
	as := &AttackerStats{
		name:              asw.Name,
		numRequests:       asw.Requests,
		numFailures:       asw.Failures,
		totalResponseTime: msToNS(asw.TotalResponseTime),
		minResponseTime:   msToNS(asw.MinResponseTime),
		maxResponseTime:   msToNS(asw.MaxResponseTime),
		trendSuccess:      &LimitedSizeMap{content: asw.TrendSuccess, size: 30},
		trendFailures:     asw.TrendFailures,
		responseTimes:     map[roundedMillisecond]int64{},
		failuresTimes:     asw.FailureTimes,
		startTime:         timeFromTimestamp(asw.StartTime),
		lastRequestTime:   timeFromTimestamp(asw.LastRequestTime),
		interval:          CurrentQPSTimeRange,
	}
	for k, v := range asw.ResponseTimes {
		as.responseTimes[roundedMillisecond(k)] = v
	}
	return as
}

func NewAttackerStats(n string) *AttackerStats {
	return &AttackerStats{
		name:          n,
		trendSuccess:  newLimitedSizeMap(30),
		trendFailures: map[int64]int64{},
		responseTimes: map[roundedMillisecond]int64{},
		failuresTimes: map[string]int64{},
		interval:      CurrentQPSTimeRange,
	}
}

func newLimitedSizeMap(s int64) *LimitedSizeMap {
	return &LimitedSizeMap{
		content: map[int64]int64{},
		size:    s,
	}
}

func (ae *AttackerError) Error() string {
	return ae.CausedBy
}

// totalQPS 获取总的QPS
func (as *AttackerStats) totalQPS() float64 {
	return float64(as.numRequests) / as.lastRequestTime.Sub(as.startTime).Seconds()
}

// currentQPS 最近12秒的QPS
func (as *AttackerStats) currentQPS() float64 {
	if as.startTime.IsZero() || as.lastRequestTime.IsZero() {
		Logger.Warn("no start or end time")
		return 0
	}

	now := time.Now().Add(-time.Second)            // 当前一秒可能未完成，统计时，往前推一秒
	start := now.Add(-(as.interval - time.Second)) // 比如当前15秒，往回推5秒，起点是11秒而不是10秒

	if start.Before(as.startTime) {
		start = as.startTime
	}

	//if now.Unix() == start.Unix() {
	//	return 0 // 相减会是0，不处理这种情况
	//}

	var total int64

	for i := start.Unix(); i <= now.Unix(); i++ {
		if v, ok := as.trendSuccess.content[i]; ok {
			total += v
		}
	}
	if total == 0 {
		Logger.Warn("empty total value", zap.Int64("now", now.Unix()), zap.Int64("start", start.Unix()),
			zap.Any("trend", as.trendSuccess))
		return 0
	}
	return float64(total) / float64(now.Unix()-start.Unix()+1)
}

func (ls *LimitedSizeMap) accumulate(k, v int64) {
	// 调用方自身来保证线程安全
	if _, ok := ls.content[k]; ok {
		ls.content[k] += v
	} else {
		for key, _ := range ls.content {
			if (k - key) > ls.size {
				delete(ls.content, key)
			}
		}
		ls.content[k] = v
	}
}

func (as *AttackerStats) logSuccess(ret *Result) {
	as.lock.Lock()
	defer as.lock.Unlock()

	now := time.Now()
	t := time.Duration(ret.Duration)
	atomic.AddInt64(&as.numRequests, 1)

	if as.startTime.IsZero() {
		as.startTime = now
		as.minResponseTime = t
	}
	as.lastRequestTime = now

	if t < as.minResponseTime {
		as.minResponseTime = t
	}
	if t > as.maxResponseTime {
		as.maxResponseTime = t
	}

	as.totalResponseTime += t
	//as.trendSuccess[now.Unix()]++
	as.trendSuccess.accumulate(now.Unix(), 1)
	as.responseTimes[timeDurationToRoundedMillisecond(t)]++
}

func (as *AttackerStats) logFailure(ret *Result) {
	as.lock.Lock()
	defer as.lock.Unlock()

	now := time.Now()
	if as.startTime.IsZero() {
		as.startTime = now
		as.minResponseTime = time.Duration(ret.Duration)
	}
	as.lastRequestTime = now

	atomic.AddInt64(&as.numFailures, 1)
	as.failuresTimes[ret.Error.Error()]++
	as.trendFailures[now.Unix()]++
}

func (as *AttackerStats) log(ret *Result) {
	if ret.Error == nil {
		as.logSuccess(ret) // 请求成功
		return
	}
	as.logFailure(ret) // 请求失败
}

// percentile 获取x%的响应时间
func (as *AttackerStats) percentile(f float64) time.Duration {
	if f <= 0.0 {
		return as.minResponseTime
	}

	if f >= 1.0 {
		return as.maxResponseTime
	}

	hit := int64(float64(as.numRequests)*f + .5)
	if hit == as.numRequests {
		return as.maxResponseTime
	}

	times := []int{}
	for k := range as.responseTimes {
		times = append(times, int(k))
	}

	// sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	sort.Ints(times)

	for _, val := range times {
		counts := as.responseTimes[roundedMillisecond(val)]
		hit -= counts
		if hit <= 0 {
			return roundedMillisecondToDuration(roundedMillisecond(val))
		}
	}
	Logger.Error("unreachable code")
	return ZeroDuration // occur error
}

// min 最快响应时间
func (as *AttackerStats) min() time.Duration {
	return as.minResponseTime
}

// max 最慢响应时间
func (as *AttackerStats) max() time.Duration {
	return as.maxResponseTime
}

// average 平均响应时间
func (as *AttackerStats) average() time.Duration {
	if as.numRequests == 0 {
		return ZeroDuration
	}
	return time.Duration(int64(as.totalResponseTime) / int64(as.numRequests))
}

// median 响应时间中位数
func (as *AttackerStats) median() time.Duration {
	return as.percentile(.5)
}

// failRatio 错误率
func (as *AttackerStats) failRatio() float64 {
	total := as.numFailures + as.numRequests
	if total == 0 {
		return 0.0
	}
	return float64(as.numFailures) / float64(total)
}

// helper 打印统计结果
func (as *AttackerStats) report(full bool) *AttackerReport {
	as.lock.RLock()
	defer as.lock.RUnlock()

	r := &AttackerReport{
		Name:           as.name,
		Requests:       atomic.LoadInt64(&as.numRequests),
		Failures:       atomic.LoadInt64(&as.numFailures),
		Min:            timeDurationToMillisecond(as.min()),
		Max:            timeDurationToMillisecond(as.max()),
		Median:         timeDurationToMillisecond(as.median()),
		Average:        timeDurationToMillisecond(as.average()),
		Distributions:  map[string]int64{},
		FailRatio:      strconv.FormatFloat(as.failRatio()*100, 'f', 2, 64) + " %%",
		FailureDetails: as.failuresTimes,
		FullHistory:    full,
	}

	if full {
		r.QPS = int64(as.totalQPS())
	} else {
		r.QPS = int64(as.currentQPS())
	}

	for _, percent := range timeDistributions {
		r.Distributions[strconv.FormatFloat(percent, 'f', 2, 64)] = timeDurationToMillisecond(as.percentile(percent))
	}
	return r
}

func (as *AttackerStats) Aggregate(another *AttackerStats) *AttackerStats {
	as.lock.Lock()
	defer as.lock.Unlock()

	as.numRequests += another.numRequests
	as.numFailures += another.numFailures
	as.totalResponseTime += another.totalResponseTime
	if as.minResponseTime > another.minResponseTime {
		as.minResponseTime = another.minResponseTime
	}
	if as.maxResponseTime < another.maxResponseTime {
		as.maxResponseTime = another.maxResponseTime
	}

	switch {
	case as.trendSuccess == nil || as.trendSuccess.content == nil && another.trendSuccess != nil && another.trendSuccess.content != nil:
		as.trendSuccess = another.trendSuccess
	case as.trendSuccess == nil || as.trendSuccess.content == nil && another.trendSuccess != nil && another.trendSuccess.content == nil:
		as.trendSuccess = newLimitedSizeMap(30)
	case as.trendSuccess == nil || as.trendSuccess.content == nil && another.trendSuccess == nil:
		as.trendSuccess = newLimitedSizeMap(30)
	default:
		for k, v := range another.trendSuccess.content {
			as.trendSuccess.content[k] += v
		}
	}

	switch {
	case as.trendFailures == nil && another.trendFailures != nil:
		as.trendFailures = another.trendFailures
	case as.trendFailures == nil && another.trendFailures == nil:
		as.trendFailures = map[int64]int64{}
	default:
		for k, v := range another.trendFailures {
			as.trendFailures[k] += v
		}
	}

	switch {
	case as.responseTimes == nil && another.responseTimes != nil:
		as.responseTimes = another.responseTimes
	case as.responseTimes == nil && another.responseTimes == nil:
		as.responseTimes = map[roundedMillisecond]int64{}
	default:
		for k, v := range another.responseTimes {
			as.responseTimes[k] += v
		}
	}

	switch {
	case as.failuresTimes == nil && another.failuresTimes != nil:
		as.failuresTimes = another.failuresTimes
	case as.failuresTimes == nil && another.failuresTimes == nil:
		as.failuresTimes = map[string]int64{}
	default:
		for k, v := range another.failuresTimes {
			as.failuresTimes[k] = v
		}
	}

	if another.startTime.Before(as.startTime) {
		as.startTime = another.startTime
	}
	if another.lastRequestTime.After(as.lastRequestTime) {
		as.lastRequestTime = another.lastRequestTime
	}
	return as
}

func (as *AttackerStats) BatchAggregate(others ...*AttackerStats) *AttackerStats {
	for _, another := range others {
		as.Aggregate(another)
	}
	return as
}

func (as *AttackerStats) Wrap() *pb.AttackerStatsWrapper {
	as.lock.RLock()
	defer as.lock.RUnlock()
	asw := &pb.AttackerStatsWrapper{
		Name:              as.name,
		Requests:          as.numRequests,
		Failures:          as.numFailures,
		TotalResponseTime: as.totalResponseTime.Nanoseconds() / 1e6,
		MinResponseTime:   as.minResponseTime.Nanoseconds() / 1e6,
		MaxResponseTime:   as.maxResponseTime.Nanoseconds() / 1e6,
		TrendSuccess:      map[int64]int64{},
		TrendFailures:     map[int64]int64{},
		ResponseTimes:     map[int64]int64{},
		FailureTimes:      map[string]int64{},
		StartTime:         as.startTime.UnixNano() / 1e6,
		LastRequestTime:   as.lastRequestTime.UnixNano() / 1e6,
	}
	for k, v := range as.responseTimes {
		asw.ResponseTimes[int64(k)] = v
	}

	for k, v := range as.trendSuccess.content {
		asw.TrendSuccess[k] = v
	}

	for k, v := range as.trendFailures {
		asw.TrendFailures[k] = v
	}

	for k, v := range as.failuresTimes {
		asw.FailureTimes[k] = v
	}
	return asw
}

func NewAndBatchAggregate(ass ...*AttackerStats) *AttackerStats {
	if ass == nil {
		return nil
	}
	name := ass[0].name
	return NewAttackerStats(name).BatchAggregate(ass...)
}

func NewSummaryStats() *SummaryStats {
	return &SummaryStats{}
}

func (ss *SummaryStats) Log(ret *Result) {
	val, _ := ss.nodes.LoadOrStore(ret.Name, NewAttackerStats(ret.Name))
	val.(*AttackerStats).log(ret)
}

func (ss *SummaryStats) Report(full bool) Report {
	rep := map[string]*AttackerReport{}

	ss.nodes.Range(func(key, value interface{}) bool {
		rep[key.(string)] = value.(*AttackerStats).report(full)
		return true
	})

	return rep
}

func (ss *SummaryStats) Store(stats *AttackerStats) {
	ss.nodes.Store(stats.name, stats)
}

func (ss *SummaryStats) Aggregate(another *SummaryStats) {
	another.nodes.Range(func(key, value interface{}) bool {
		original, loaded := ss.nodes.LoadOrStore(key, value)
		if loaded {
			original.(*AttackerStats).Aggregate(value.(*AttackerStats))
		}
		return true
	})
}

func (ss *SummaryStats) IsZero() bool {
	isZero := true
	ss.nodes.Range(func(key, value interface{}) bool {
		if value.(*AttackerStats) != nil {
			isZero = false
			return true
		}
		return true
	})
	return isZero
}

func (ss *SummaryStats) ToAttackerStatsWrappers() []*pb.AttackerStatsWrapper {
	var ret []*pb.AttackerStatsWrapper
	ss.nodes.Range(func(_, value interface{}) bool {
		ret = append(ret, value.(*AttackerStats).Wrap())
		return true
	})
	return ret
}

func NewAttackerError(name string, err error) *AttackerError {
	return &AttackerError{Name: name, CausedBy: err.Error()}
}

func NewResult(n string, d time.Duration, err error) *Result {
	if err == nil {
		return &Result{Name: n, Duration: int64(d)}
	}
	return &Result{Name: n, Duration: int64(d), Error: NewAttackerError(n, err)}
}
