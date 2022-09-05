package replayer

import (
	"github.com/wosai/havok/internal/logger"
	"math/rand"
	"time"

	"github.com/json-iterator/go"
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
