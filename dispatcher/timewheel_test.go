package dispatcher

import (
	"testing"
	"time"

	pb "github.com/wosai/havok/protobuf"
	"github.com/stretchr/testify/assert"
)

var (
	replayBegin int64 = 1405544146132
	replayEnd         = replayBegin + int64(time.Minute.Nanoseconds()/1e6)
)

func genLogRecord(msec int64) *LogRecordWrapper {
	return &LogRecordWrapper{
		OccurAt: ParseMSec(msec),
	}
}

func TestTimeWheel_Wheel(t *testing.T) {
	tw, _ := NewTimeWheel(&pb.JobConfiguration{
		Rate:  1.0,
		Begin: replayBegin,
		End:   replayEnd,
		Speed: 4.0,
	})
	tw.WithHavok(DefaultHavok)

	go func(w *TimeWheel) {
		w.Recv() <- genLogRecord(replayBegin - 1)
		w.Recv() <- genLogRecord(replayBegin)
		w.Recv() <- genLogRecord(replayBegin + int64(3*time.Second.Nanoseconds()/1e6))
		w.Recv() <- genLogRecord(replayBegin + int64(13*time.Second.Nanoseconds()/1e6))
		w.Recv() <- genLogRecord(replayBegin + int64(31*time.Second.Nanoseconds()/1e6))
		w.Recv() <- genLogRecord(replayBegin + int64(33*time.Second.Nanoseconds()/1e6))
		w.Recv() <- genLogRecord(replayBegin + int64(50*time.Second.Nanoseconds()/1e6))
		w.Recv() <- genLogRecord(replayBegin + int64(51*time.Second.Nanoseconds()/1e6))
		w.Recv() <- genLogRecord(replayBegin + int64(51*time.Second.Nanoseconds()/1e6))
		w.Recv() <- genLogRecord(replayEnd)
		w.Recv() <- genLogRecord(replayBegin + int64(61*time.Second.Nanoseconds()/1e6))
	}(tw)

	start := time.Now()
	tw.Start()
	d := time.Since(start)
	assert.True(t, d < 16*time.Second)
}
