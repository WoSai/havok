package fetcher

import (
	"container/list"
	"context"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	testing2 "github.com/wosai/havok/internal/pkg/fetcher/testing"
	pb "github.com/wosai/havok/pkg/genproto"
	"github.com/wosai/havok/pkg/plugin"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func TestMultiFetcher_Fetch(t *testing.T) {

	type logs struct {
		num       int
		returnErr error
	}

	for _, tc := range []struct {
		name      string
		actual    []logs
		expected  int
		returnErr error
	}{
		{
			name: "Number of logs: 0, 1",
			actual: []logs{
				{num: 0, returnErr: nil},
				{num: 1, returnErr: nil},
			},
			expected:  1,
			returnErr: nil,
		},
		{
			name: "Number of logs: 1, 0",
			actual: []logs{
				{num: 1, returnErr: nil},
				{num: 0, returnErr: nil},
			},
			expected:  1,
			returnErr: nil,
		},
		{
			name: "Number of logs all 0",
			actual: []logs{
				{num: 0, returnErr: nil},
				{num: 0, returnErr: nil},
			},
			expected:  0,
			returnErr: nil,
		},
		{
			name: "Number of logs: 1, 10, 0, 10",
			actual: []logs{
				{num: 1, returnErr: nil},
				{num: 10, returnErr: nil},
				{num: 0, returnErr: nil},
				{num: 10, returnErr: nil},
			},
			returnErr: nil,
			expected:  21,
		},
		{
			name: "Number of logs: 0, 0, 0, 10",
			actual: []logs{
				{num: 0, returnErr: nil},
				{num: 0, returnErr: nil},
				{num: 0, returnErr: nil},
				{num: 10, returnErr: nil},
			},
			returnErr: nil,
			expected:  10,
		},
		{
			name: "Number of logs: 1, 10",
			actual: []logs{
				{num: 1, returnErr: io.EOF},
				{num: 1000, returnErr: nil},
			},
			returnErr: io.EOF,
		},
		{
			name: "Number of logs: 1, 10",
			actual: []logs{
				{num: 1, returnErr: io.ErrUnexpectedEOF},
				{num: 100, returnErr: io.ErrUnexpectedEOF},
			},
			returnErr: io.ErrUnexpectedEOF,
		},
	} {
		mf := &MultiFetcher{sortLogs: list.New()}
		for i, log := range tc.actual {
			ctl := gomock.NewController(t)
			f := testing2.NewMockFetcher(ctl)
			f.EXPECT().Name().Return("mock fetcher").AnyTimes()
			f.EXPECT().Fetch(gomock.Any(), gomock.Any()).
				DoAndReturn(fetchFunc(i, log.num, log.returnErr))
			mf.fetchers = append(mf.fetchers, f)
		}
		var (
			ctx     = context.Background()
			output  = make(chan *pb.LogRecord)
			count   int
			outputs []*pb.LogRecord
			wg      sync.WaitGroup
		)

		wg.Add(1)
		go func() {
			defer wg.Done()
			for log := range output {
				count++
				outputs = append(outputs, log)
			}
		}()

		err := mf.Fetch(ctx, output)
		assert.Equal(t, tc.returnErr, err)
		wg.Wait()
		if err == nil {
			// 断言log数量相等
			assert.Equal(t, tc.expected, count)
		}
		// 断言log是有序的
		assert.True(t, sort.SliceIsSorted(outputs, func(i, j int) bool {
			return outputs[i].OccurAt.AsTime().Before(outputs[j].OccurAt.AsTime())
		}))
	}
}

func BenchmarkMultiFetcher_Fetch_1_fetcher(b *testing.B) {
	mf := NewMultiFetcher().(*MultiFetcher)
	mf.fetchers = append(mf.fetchers, &fakeFetcher{fetch: fetchFunc(1, -1, nil)})

	var (
		ctx    = context.Background()
		output = make(chan *pb.LogRecord)
	)

	b.StartTimer()
	go func() {
		err := mf.Fetch(ctx, output)
		if err != nil {
			b.Error(err)
		}
	}()

	i := 0
	for range output {
		i++
		if i >= b.N {
			return
		}
	}
}

func BenchmarkMultiFetcher_Fetch_5_fetcher(b *testing.B) {
	mf := NewMultiFetcher().(*MultiFetcher)
	for i := 0; i < 5; i++ {
		mf.fetchers = append(mf.fetchers, &fakeFetcher{fetch: fetchFunc(i, -1, nil)})
	}

	var (
		ctx    = context.Background()
		output = make(chan *pb.LogRecord)
	)

	b.StartTimer()
	go func() {
		err := mf.Fetch(ctx, output)
		if err != nil {
			b.Error(err)
		}
	}()

	i := 0
	for range output {
		i++
		if i >= b.N {
			return
		}
	}
}

func BenchmarkMultiFetcher_Fetch_50_fetcher(b *testing.B) {
	mf := NewMultiFetcher().(*MultiFetcher)
	for i := 0; i < 50; i++ {
		mf.fetchers = append(mf.fetchers, &fakeFetcher{fetch: fetchFunc(i, -1, nil)})
	}

	var (
		ctx    = context.Background()
		output = make(chan *pb.LogRecord)
	)

	go func() {
		err := mf.Fetch(ctx, output)
		if err != nil {
			b.Error(err)
		}
	}()

	b.StartTimer()

	i := 0
	for range output {
		i++
		if i >= b.N {
			return
		}
	}
}

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

type (
	fakeFetcher struct {
		fetch func(ctx context.Context, output chan<- *pb.LogRecord) error
	}
)

func (f *fakeFetcher) Name() string {
	return "fake-fetcher"
}

func (f *fakeFetcher) Apply(any) {}

func (f *fakeFetcher) WithDecoder(record plugin.LogDecoder) {}

func (f *fakeFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	return f.fetch(ctx, output)
}

func fetchFunc(id int, num int, err error) func(ctx context.Context, output chan<- *pb.LogRecord) error {
	var second int64 = 1
	var count int
	return func(ctx context.Context, output chan<- *pb.LogRecord) error {
		defer close(output)
		for num >= 0 && count < num {
			if num >= 0 && count >= num {
				break
			}

			second = second + rand.Int63n(10)
			select {
			case output <- &pb.LogRecord{
				Url:     fmt.Sprintf("https://github.com/%d", id),
				Method:  "get",
				OccurAt: &timestamppb.Timestamp{Seconds: second},
			}:
				count++
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return err
	}
}
