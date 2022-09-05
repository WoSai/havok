package replayer

import (
	pb "github.com/wosai/havok/protobuf"
	"github.com/wosai/havok/types"
	"go.uber.org/zap"
	"time"
)

type (
	RunnerFlag struct {
		stats *types.SummaryStats
	}
)

var (
	runnerFlag = &RunnerFlag{stats: types.NewSummaryStats()}
)

const (
	replayerCurrentConcurrency = "replayer current concurrency"
	replayerTotalConcurrency   = "replayer total concurrency"
)

// 记录统计
func statistor() {
	for result := range resultPipeline {
		runnerFlag.stats.Log(result)
	}
}

// 上交统计报告
func submitter(replayer *replayer) {
	performanceStats := make(map[string]float64)
	performanceStats[replayerTotalConcurrency] = float64(replayer.Concurrency)
	for stats := range submitterPipeline {
		summaryStats := runnerFlag.stats.ToAttackerStatsWrappers()
		performanceStats[replayerCurrentConcurrency] = float64(len(replayer.ch))
		Logger.Info("send stats request", zap.Any("data", summaryStats), zap.Any("performance", performanceStats))
		sr := &pb.StatsReport{ReplayerId: DefaultReplayerId, ReportTime: time.Now().UnixNano() / 1e6, RequestId: stats.RequestId, Stats: summaryStats, PerformanceStats: performanceStats}
		reportorPipeline <- sr
	}
}

func Runner(replayer *replayer) {
	go statistor()
	go submitter(replayer)
}
