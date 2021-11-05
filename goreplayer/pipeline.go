package replayer

import (
	pb "github.com/WoSai/havok/protobuf"
	"github.com/WoSai/havok/types"
)

type (
	ReplayerPipeline  chan *pb.LogRecord
	ResultPipeline    chan *types.Result
	SubmitterPipeline chan *pb.StatsRequest
	ReportorPipeline  chan *pb.StatsReport
)

var (
	replayerPipeline  ReplayerPipeline
	resultPipeline    ResultPipeline
	submitterPipeline SubmitterPipeline
	reportorPipeline  ReportorPipeline

	replayerPipelineSize  = 2000
	resultPipelineSize    = 1000
	submitterPipelineSize = 3
	reportorPipelineSize  = 3
)

func newReplayerPipeline(size int) ReplayerPipeline {
	return make(chan *pb.LogRecord, size)
}

func newResultPipeline(size int) ResultPipeline {
	return make(chan *types.Result, size)
}

func newSubmitterPipeline(size int) SubmitterPipeline {
	return make(chan *pb.StatsRequest, size)
}

func newReportorPipeline(size int) ReportorPipeline {
	return make(chan *pb.StatsReport, size)
}
