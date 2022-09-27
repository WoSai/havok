package fetcher

import (
	"container/list"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	pb "github.com/wosai/havok/pkg/genproto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestInsertSortedLogs(t *testing.T) {

	for _, tc := range []struct {
		name     string
		actual   []*indexLog
		insert   *indexLog
		expected []*indexLog
	}{
		{
			name: "insert middle",
			actual: []*indexLog{
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 2}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 3}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 5}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 6}}},
			},
			insert: &indexLog{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 4}}},
			expected: []*indexLog{
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 2}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 3}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 4}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 5}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 6}}},
			},
		},
		{
			name: "insert begin",
			actual: []*indexLog{
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 7}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 7}}},
			},
			insert: &indexLog{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 6}}},
			expected: []*indexLog{
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 6}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 7}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 7}}},
			},
		},
		{
			name: "insert end",
			actual: []*indexLog{
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 19}}},
			},
			insert: &indexLog{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 20}}},
			expected: []*indexLog{
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 19}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 20}}},
			},
		},
		{
			name:   "insert into empty slice",
			actual: []*indexLog{},
			insert: &indexLog{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 6}}},
			expected: []*indexLog{
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 6}}},
			},
		},
	} {

		logs := list.New()
		for _, log := range tc.actual {
			logs.PushBack(log)
		}

		insertSortedLogs(logs, tc.insert)

		sortLogs := []*indexLog{}
		for e := logs.Front(); e != nil; e = e.Next() {
			sortLogs = append(sortLogs, e.Value.(*indexLog))
		}

		assert.True(t, sort.SliceIsSorted(sortLogs, func(i, j int) bool {
			return sortLogs[i].OccurAt.AsTime().Before(sortLogs[j].OccurAt.AsTime())
		}))
		assert.Equal(t, sortLogs, tc.expected)
	}
}

func BenchmarkInsertSortLogs(b *testing.B) {
	x := list.New()
	for i := 0; i < 5; i++ {
		x.PushBack(&indexLog{LogRecord: &pb.LogRecord{
			OccurAt: &timestamppb.Timestamp{Seconds: int64(i)},
		}})
	}
	for i := 0; i < b.N; i++ {
		insertSortedLogs(x, &indexLog{LogRecord: &pb.LogRecord{
			OccurAt: &timestamppb.Timestamp{Seconds: rand.Int63n(5)},
		}})
		x.Remove(x.Back())
	}
}
