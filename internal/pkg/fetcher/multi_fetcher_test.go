package fetcher

import (
	"container/list"
	"context"
	"fmt"
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

	for _, tc := range []struct {
		name     string
		actual   []int
		expected int
	}{
		{
			name:     "Number of logs: 0, 1",
			actual:   []int{0, 1},
			expected: 1,
		},
		{
			name:     "Number of logs: 1, 0",
			actual:   []int{1, 0},
			expected: 1,
		},
		{
			name:     "Number of logs all 0",
			actual:   []int{0, 0},
			expected: 0,
		},
		{
			name:     "Number of logs: 1, 10, 0, 10",
			actual:   []int{1, 10, 0, 10},
			expected: 21,
		},
		{
			name:     "Number of logs: 0, 0, 0, 10",
			actual:   []int{0, 0, 0, 10},
			expected: 10,
		},
	} {
		mf := &MultiFetcher{sortLogs: list.New()}
		for i, num := range tc.actual {
			ctl := gomock.NewController(t)
			f := testing2.NewMockFetcher(ctl)
			f.EXPECT().Fetch(gomock.Any(), gomock.Any()).
				DoAndReturn(fetchFunc(i, num))
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
		assert.Nil(t, err)
		wg.Wait()
		// 断言log数量相等
		assert.Equal(t, tc.expected, count)
		// 断言log是有序的
		for range outputs {
		}

		assert.True(t, sort.SliceIsSorted(outputs, func(i, j int) bool {
			return outputs[i].OccurAt.AsTime().Before(outputs[j].OccurAt.AsTime())
		}))
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

func BenchmarkMultiFetcher_Fetch_1_fetcher(b *testing.B) {
	mf := NewMultiFetcher().(*MultiFetcher)
	mf.fetchers = append(mf.fetchers, &fakeFetcher{fetch: fetchFunc(1, -1)})

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
		mf.fetchers = append(mf.fetchers, &fakeFetcher{fetch: fetchFunc(i, -1)})
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
		mf.fetchers = append(mf.fetchers, &fakeFetcher{fetch: fetchFunc(i, -1)})
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

func fetchFunc(id int, num int) func(ctx context.Context, output chan<- *pb.LogRecord) error {
	var second int64 = 1
	var count int
	return func(ctx context.Context, output chan<- *pb.LogRecord) error {
		defer close(output)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if num >= 0 && count >= num {
				break
			}
			second = second + rand.Int63n(10)
			output <- &pb.LogRecord{
				Url:     fmt.Sprintf("https://github.com/%d", id),
				Method:  "get",
				OccurAt: &timestamppb.Timestamp{Seconds: second},
			}
			count++
		}
		return nil
	}
}

func TestInsertSortedSlice(t *testing.T) {

	for _, tc := range []struct {
		name     string
		actual   []*indexLog
		insert   *indexLog
		expected []*indexLog
	}{
		{
			name: "",
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
			name: "",
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
			name:   "",
			actual: []*indexLog{},
			insert: &indexLog{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 6}}},
			expected: []*indexLog{
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 6}}},
			},
		},
		{
			name: "",
			actual: []*indexLog{
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 19}}},
			},
			insert: &indexLog{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 20}}},
			expected: []*indexLog{
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 19}}},
				{LogRecord: &pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: 20}}},
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

func TestChannel(t *testing.T) {
	var errCh = make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		time.Sleep(time.Second)
		defer wg.Done()
		for i := 0; i < 3; i++ {

			select {
			case errCh <- struct{}{}:
				fmt.Println("put")
			default:
				fmt.Println(1)
			}
		}
	}()
	defer wg.Wait()

	for {
		select {
		case err := <-errCh:
			fmt.Println("error", err)
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
}

func TestName2(t *testing.T) {
	var ch = make(chan struct{})

	close(ch)

	v := <-ch
	fmt.Println(v)
}
