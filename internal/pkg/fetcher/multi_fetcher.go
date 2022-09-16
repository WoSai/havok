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
		f := iplugin.LookupFetcher(cfg)
		mf.fetchers = append(mf.fetchers, f)
	}
}

func (mf *MultiFetcher) WithDecoder(decoder plugin.LogDecoder) {

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

	for {
		// exit when all logs are read
		if len(mf.sortLogs) == 0 {
			return nil
		}

		minLog := mf.sortLogs[0]
		output <- minLog.LogRecord
		minIdx := minLog.idx
		mf.sortLogs = mf.sortLogs[1:]

		select {
		case c, ok := <-mf.rChannels[minIdx]:
			if !ok {
				continue
			}

			log := &indexLog{
				LogRecord: c,
				idx:       minIdx,
			}
			mf.sortLogs = insertSortedLogs(mf.sortLogs, log, func(i int) bool {
				return log.OccurAt.AsTime().Before(mf.sortLogs[i].OccurAt.AsTime())
			})
		}
	}
}

func (mf *MultiFetcher) insertSortedLogs(log *indexLog, isInsert func(i int) bool) {
	for i := 0; i < len(mf.sortLogs)-1; i++ {
		if log.idx < mf.sortLogs[i].idx {
			mf.sortLogs = append(mf.sortLogs, log)
			copy(mf.sortLogs[i+1:], mf.sortLogs[i:])
			mf.sortLogs[i] = log

			return
		}
	}
	mf.sortLogs = append(mf.sortLogs, log)
}

func (mf *MultiFetcher) cancelReadFetchers() {
	mf.cancel()
}

func init() {
	iplugin.Register(NewMultiFetcher())
}

func insertSortedLogs(x []*indexLog, log *indexLog, insert func(i int) bool) []*indexLog {
	for i := 0; i < len(x); i++ {
		if insert(i) {
			x = append(x, log)
			copy(x[i+1:], x[i:])
			x[i] = log

			return x
		}
	}
	return append(x, log)
}
