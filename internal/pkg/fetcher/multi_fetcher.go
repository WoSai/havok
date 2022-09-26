package fetcher

import (
	"context"
	"encoding/json"
	"sync"

	iplugin "github.com/wosai/havok/internal/plugin"
	"github.com/wosai/havok/logger"
	pb "github.com/wosai/havok/pkg/genproto"
	"github.com/wosai/havok/pkg/plugin"
	"go.uber.org/zap"
)

type (
	MultiFetcher struct {
		wg       sync.WaitGroup
		cancel   context.CancelFunc
		fetchers []plugin.Fetcher
		mergeSrv mergeService
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

func NewMultiFetcher(merge mergeService) plugin.Fetcher {
	return &MultiFetcher{
		fetchers: []plugin.Fetcher{},
		mergeSrv: merge,
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

	var option = new(MultiOption)
	err = json.Unmarshal(b, option)
	if err != nil {
		panic(err)
	}
	logger.Logger.Info("apply fetcher config", zap.String("name", mf.Name()), zap.Any("config", option))

	if len(option.Fetchers) == 0 {
		panic(mf.Name() + " invalid option: " + "fetchers not be empty")
	}

	for _, cfg := range option.Fetchers {
		f := iplugin.LookupFetcher(cfg)
		mf.WithFetcher(f)
	}
}

func (mf *MultiFetcher) WithFetcher(f ...plugin.Fetcher) {
	mf.fetchers = append(mf.fetchers, f...)
}

func (mf *MultiFetcher) WithDecoder(decoder plugin.LogDecoder) {

}

func (mf *MultiFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	subCtx, cancel := context.WithCancel(ctx)
	mf.cancel = cancel

	var fetchError = make(chan error, len(mf.fetchers))
	var mergeError = make(chan error, 1)

	mf.wg.Add(len(mf.fetchers))
	for i, _ := range mf.fetchers {
		ch := make(chan *pb.LogRecord, 100)
		mf.mergeSrv.Merge(ch)

		go func(ctx context.Context, i int, ch chan *pb.LogRecord) {
			defer mf.wg.Done()
			err := mf.fetchers[i].Fetch(ctx, ch)
			if err != nil {
				logger.Logger.Error("sub fetcher Fetch fail", zap.String("name", mf.fetchers[i].Name()), zap.Error(err))
				fetchError <- err
			}
		}(subCtx, i, ch)
	}

	mf.wg.Add(1)
	go func() {
		defer mf.wg.Done()
		err := mf.mergeSrv.Output(subCtx, output)
		if err != nil {
			logger.Logger.Error("merge service fail", zap.Error(err))
		}
		mergeError <- err
	}()

	select {
	case <-ctx.Done():
		mf.wg.Wait()
		return ctx.Err()
	case err := <-fetchError:
		mf.cancel()
		mf.wg.Wait()
		return err
	case err := <-mergeError:
		if err != nil {
			mf.cancel()
			mf.wg.Wait()
			return err
		}
		mf.wg.Wait()
		return nil
	}
}

func init() {
	iplugin.Register(NewMultiFetcher(newMergeService()))
}
