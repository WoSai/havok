package fetcher

import (
	"context"
	"encoding/json"
	"sort"

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
		sortLogs  []*indexLog
		cancel    context.CancelFunc
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
	logger.Logger.Info("apply fetcher config", zap.String("name", mf.Name()), zap.Any("config", option))

	for _, cfg := range option.Fetchers {
		f := iplugin.LookupFetcher(cfg.Name)
		f.Apply(cfg.Args)
		if len(cfg.Needs) > 1 {
			panic(mf.Name() + " invalid option: fetcher.needs > 1")
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
	defer close(output)
	mf.readFetchers(ctx)
	defer mf.cancelReadFetchers()

	mf.prepareSortLogs()
	err := mf.loop(ctx, output)
	return err
}

func (mf *MultiFetcher) readFetchers(ctx context.Context) {
	mf.rChannels = make([]chan *pb.LogRecord, len(mf.fetchers))
	cancelCtx, cancel := context.WithCancel(ctx)
	mf.cancel = cancel

	for i, _ := range mf.fetchers {
		ch := make(chan *pb.LogRecord, 10)
		mf.rChannels[i] = ch

		go func(ctx context.Context, i int) {
			err := mf.fetchers[i].Fetch(ctx, mf.rChannels[i])
			if err != nil {
				logger.Logger.Error("sub fetcher Fetch fail", zap.String("name", mf.fetchers[i].Name()), zap.Error(err))
			}
		}(cancelCtx, i)
	}
}

func (mf *MultiFetcher) prepareSortLogs() {
	for i, subCh := range mf.rChannels {
		val, ok := <-subCh
		if ok {
			mf.sortLogs = append(mf.sortLogs, &indexLog{
				LogRecord: val,
				idx:       i,
			})
		}
	}
	sort.Slice(mf.sortLogs, func(i, j int) bool {
		return mf.sortLogs[i].OccurAt.AsTime().Before(mf.sortLogs[j].OccurAt.AsTime())
	})
}

// read from multi fetcher --> sort logs --> write into channel
func (mf *MultiFetcher) loop(ctx context.Context, output chan<- *pb.LogRecord) error {
start:
	// exit when all logs are read
	if len(mf.sortLogs) == 0 {
		return nil
	}

	output <- mf.sortLogs[0].LogRecord
	minIdx := mf.sortLogs[0].idx

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c, ok := <-mf.rChannels[minIdx]:
			// remove the indexLog when channel is closed
			if !ok {
				mf.sortLogs = mf.sortLogs[1:]
				goto start
			}
			if len(mf.sortLogs) > 1 && c.OccurAt.AsTime().After(mf.sortLogs[1].OccurAt.AsTime()) {
				mf.sortLogs[0] = &indexLog{
					LogRecord: c,
					idx:       minIdx,
				}
				// TODO 可以针对大量fetcher做优化
				sort.Slice(mf.sortLogs, func(i, j int) bool {
					return mf.sortLogs[i].OccurAt.AsTime().Before(mf.sortLogs[j].OccurAt.AsTime())
				})
				goto start
			}
			output <- c
		}
	}
}

func (mf *MultiFetcher) cancelReadFetchers() {
	mf.cancel()
}

func init() {
	iplugin.Register(NewMultiFetcher())
}
