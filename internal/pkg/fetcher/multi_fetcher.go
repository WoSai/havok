package fetcher

import (
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
		fetchers  []plugin.Fetcher
		rChannels []chan *pb.LogRecord
		wChannel  chan<- *pb.LogRecord
		sortLogs  []*indexLog
		cancels   []context.CancelFunc
		wg        sync.WaitGroup
	}

	MultiOption struct {
		Fetchers []SubFetcherOption `json:"fetchers"`
	}

	SubFetcherOption struct {
		Name  string   `json:"name"`
		Args  any      `json:"args"`
		Needs []string `json:"needs,omitempty"`
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
		sortLogs: []*indexLog{},
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

	for _, cfg := range option.Fetchers {
		f := iplugin.LookupFetcher(cfg.Name)
		f.Apply(cfg.Args)
		if len(cfg.Needs) > 1 {
			panic("invalid plugin option: fetcher.needs > 1")
		} else if len(cfg.Needs) == 1 {
			f.WithDecoder(iplugin.LookupDecoder(cfg.Needs[0]))
		}
		mf.fetchers = append(mf.fetchers, f)
	}
}

func (mf *MultiFetcher) WithDecoder(decoder plugin.LogDecoder) {
	panic(mf.Name() + " don't need decoder. Please configure the decoder in sub fetcher.")
}

func (mf *MultiFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	mf.init(ctx, output)
	mf.loop(ctx)
	mf.close()

	mf.wg.Wait()
	return nil
}

func (mf *MultiFetcher) init(ctx context.Context, output chan<- *pb.LogRecord) {
	mf.wChannel = output
	mf.rChannels = make([]chan *pb.LogRecord, len(mf.fetchers))
	mf.cancels = make([]context.CancelFunc, len(mf.fetchers))
	for i, f := range mf.fetchers {
		mf.wg.Add(1)

		ch := make(chan *pb.LogRecord, 10)
		mf.rChannels[i] = ch

		ctx, cancel := context.WithCancel(ctx)
		mf.cancels[i] = cancel

		go func(ctx context.Context, ch chan *pb.LogRecord, f plugin.Fetcher) {
			defer mf.wg.Done()

			err := f.Fetch(ctx, ch)
			if err != nil {
				logger.Logger.Error("sub fetcher Fetch fail", zap.String("name", mf.fetchers[i].Name()), zap.Error(err))
			}
		}(ctx, ch, f)
	}

	for i, subCh := range mf.rChannels {
		select {
		case val, ok := <-subCh:
			if ok {
				mf.sortLogs = append(mf.sortLogs, &indexLog{
					LogRecord: val,
					idx:       i,
				})
			}
		}
	}
}

// read from multi fetcher --> sort logs --> write into channel
func (mf *MultiFetcher) loop(ctx context.Context) {
start:
	if len(mf.sortLogs) == 0 {
		return
	}
	// TODO 可以针对大量fetcher做优化
	sort.Slice(mf.sortLogs, func(i, j int) bool {
		return mf.sortLogs[i].OccurAt.AsTime().Before(mf.sortLogs[j].OccurAt.AsTime())
	})

	mf.wChannel <- mf.sortLogs[0].LogRecord
	minIdx := mf.sortLogs[0].idx

	for {
		select {
		case <-ctx.Done():
			return
		case c, ok := <-mf.rChannels[minIdx]:
			// channel is closed
			if !ok {
				mf.sortLogs = mf.sortLogs[1:]
				goto start
			}
			if len(mf.sortLogs) > 1 && c.OccurAt.AsTime().After(mf.sortLogs[1].OccurAt.AsTime()) {
				mf.sortLogs[0] = &indexLog{
					LogRecord: c,
					idx:       minIdx,
				}
				goto start
			}
			mf.wChannel <- c

		default:
		}
	}
}

func (mf *MultiFetcher) close() {
	for _, cancel := range mf.cancels {
		cancel()
	}
	close(mf.wChannel)
}

func init() {
	iplugin.Register(NewMultiFetcher())
}
