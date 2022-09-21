package fetcher

import (
	"container/list"
	"context"
	"encoding/json"
	"sort"
	"sync"

	iplugin "github.com/wosai/havok/internal/plugin"
	"github.com/wosai/havok/logger"
	pb "github.com/wosai/havok/pkg/genproto"
	"github.com/wosai/havok/pkg/plugin"
	"go.uber.org/zap"
)

type (
	MultiFetcher struct {
		wg        sync.WaitGroup
		cancel    context.CancelFunc
		fetchers  []plugin.Fetcher
		rChannels []chan *pb.LogRecord
		sortLogs  *list.List
	}

	MultiOption struct {
		Fetchers []string `json:"fetchers" yaml:"fetchers" toml:"fetchers"`
	}

	indexLog struct {
		*pb.LogRecord
		idx int
	}
)

var _ plugin.Fetcher = (*MultiFetcher)(nil)

func NewMultiFetcher() plugin.Fetcher {
	return &MultiFetcher{
		fetchers: []plugin.Fetcher{},
		sortLogs: list.New(),
	}
}

func (mf *MultiFetcher) Name() string {
	return "multi-fetcher"
}

func (mf *MultiFetcher) Apply(opt any) {
	b, err := json.Marshal(opt)
	if err != nil {
		panic(err)
	}

	var option = MultiOption{}
	err = json.Unmarshal(b, &option)
	if err != nil {
		panic(err)
	}
	logger.Logger.Info("apply fetcher config", zap.String("name", mf.Name()), zap.Any("config", option))

	for _, cfg := range option.Fetchers {
		f := iplugin.LookupFetcher(cfg)
		mf.fetchers = append(mf.fetchers, f)
	}
}

func (mf *MultiFetcher) WithDecoder(decoder plugin.LogDecoder) {

}

func (mf *MultiFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	defer close(output)

	mf.rChannels = make([]chan *pb.LogRecord, len(mf.fetchers))
	cancelCtx, cancel := context.WithCancel(ctx)
	mf.cancel = cancel

	var fetchError = make(chan error, len(mf.fetchers))

	mf.wg.Add(len(mf.fetchers))
	for i, _ := range mf.fetchers {
		ch := make(chan *pb.LogRecord, 10)
		mf.rChannels[i] = ch

		go func(ctx context.Context, i int) {
			defer mf.wg.Done()
			err := mf.fetchers[i].Fetch(ctx, mf.rChannels[i])
			if err != nil {
				logger.Logger.Error("sub fetcher Fetch fail", zap.String("name", mf.fetchers[i].Name()), zap.Error(err))
				select {
				case fetchError <- err:
					// If somebody receive fetchError, let other know the error occurred.
				default:
					//don't block
				}
			}
		}(cancelCtx, i)
	}

	defer mf.cancelReadFetchers()

	mf.prepareSortLogs()

	for mf.sortLogs.Len() > 0 {

		elem := mf.sortLogs.Front()
		mf.sortLogs.Remove(elem)

		minLog := elem.Value.(*indexLog)
		output <- minLog.LogRecord
		minIdx := minLog.idx

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-fetchError:
			return err
		case c, ok := <-mf.rChannels[minIdx]:
			if !ok {
				continue
			}

			log := &indexLog{
				LogRecord: c,
				idx:       minIdx,
			}
			insertSortedLogs(mf.sortLogs, log)
		}
	}
	return nil
}

func (mf *MultiFetcher) prepareSortLogs() {
	var logs = []*indexLog{}
	for i, subCh := range mf.rChannels {
		val, ok := <-subCh
		if ok {
			logs = append(logs, &indexLog{
				LogRecord: val,
				idx:       i,
			})
		}
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].OccurAt.AsTime().Before(logs[j].OccurAt.AsTime())
	})

	for _, log := range logs {
		mf.sortLogs.PushBack(log)
	}
}

func (mf *MultiFetcher) cancelReadFetchers() {
	mf.cancel()
	mf.wg.Wait()
}

func init() {
	iplugin.Register(NewMultiFetcher())
}

func insertSortedLogs(x *list.List, log *indexLog) {
	for e := x.Front(); e != nil; e = e.Next() {
		if log.OccurAt.AsTime().Before(e.Value.(*indexLog).OccurAt.AsTime()) {
			x.InsertBefore(log, e)
			return
		}
	}
	x.PushBack(log)
}
