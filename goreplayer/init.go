package replayer

import (
	"math/rand"
	"time"
	//"github.com/wosai/havok/logger"
	//"github.com/json-iterator/go"
	//"go.uber.org/zap"
)

func init() {
	replayerPipeline = newReplayerPipeline(replayerPipelineSize)
	resultPipeline = newResultPipeline(resultPipelineSize)
	submitterPipeline = newSubmitterPipeline(submitterPipelineSize)
	reportorPipeline = newReportorPipeline(reportorPipelineSize)

	rand.Seed(time.Now().UnixNano())
}
