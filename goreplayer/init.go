package replayer

import (
	"math/rand"
	"time"

	"github.com/json-iterator/go"
	"github.com/wosai/havok/logger"
	"go.uber.org/zap"
)

var (
	Logger *zap.Logger
	json   = jsoniter.ConfigCompatibleWithStandardLibrary
)

func init() {
	Logger = logger.Logger

	replayerPipeline = newReplayerPipeline(replayerPipelineSize)
	resultPipeline = newResultPipeline(resultPipelineSize)
	submitterPipeline = newSubmitterPipeline(submitterPipelineSize)
	reportorPipeline = newReportorPipeline(reportorPipelineSize)

	rand.Seed(time.Now().UnixNano())
}
