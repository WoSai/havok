package fetcher

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "github.com/wosai/havok/pkg/genproto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSortLogRecords(t *testing.T) {

	var logs = []*pb.LogRecord{
		{OccurAt: &timestamppb.Timestamp{Seconds: 6}},
		{OccurAt: &timestamppb.Timestamp{Seconds: 3}},
		{OccurAt: &timestamppb.Timestamp{Seconds: 8}},
	}

	sortLogRecords(logs)
	sorted := sort.SliceIsSorted(logs, func(i, j int) bool {
		return logs[i].OccurAt.AsTime().Before(logs[j].OccurAt.AsTime())
	})
	assert.True(t, sorted)
}
