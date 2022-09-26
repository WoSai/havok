package fetcher

import (
	"sort"
	"time"

	pb "github.com/wosai/havok/pkg/genproto"
)

const TimeFormat = "2006-01-02 15:04:05"

func ParseTime(s string) time.Time {
	t, err := time.Parse(TimeFormat, s)
	if err != nil {
		panic(err)
	}
	return t
}

func sortLogRecords(logs []*pb.LogRecord) {
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].OccurAt.AsTime().Before(logs[j].OccurAt.AsTime())
	})
}
