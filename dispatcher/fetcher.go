package dispatcher

import (
	"fmt"
	"github.com/wosai/havok/internal/logger"
	"github.com/wosai/havok/pkg"
	"sort"
)

type (
	multiFetcher struct {
		fetchers  []pkg.Fetcher
		rChannels []chan *pkg.LogRecordWrapper
		wChannel  chan<- *pkg.LogRecordWrapper
		sortLogs  []*indexLog
	}

	indexLog struct {
		*pkg.LogRecordWrapper
		idx int
	}
)

func NewMultiFetcher(fs ...pkg.Fetcher) *multiFetcher {
	return &multiFetcher{
		fetchers: fs,
	}
}

func (mf *multiFetcher) Read(ch chan<- *pkg.LogRecordWrapper) {
	mf.init(ch)
	go mf.start()
}

func (mf *multiFetcher) init(ch chan<- *pkg.LogRecordWrapper) {
	mf.wChannel = ch
	mf.rChannels = make([]chan *pkg.LogRecordWrapper, len(mf.fetchers))
	for i, f := range mf.fetchers {
		ch := make(chan *pkg.LogRecordWrapper, 1000)
		f.Read(ch)
		mf.rChannels[i] = ch
	}

	for i, subCh := range mf.rChannels {
		val, ok := <-subCh
		if ok {
			mf.sortLogs = append(mf.sortLogs, &indexLog{
				LogRecordWrapper: val,
				idx:              i,
			})
		}
	}
}

// read from multi fetcher --> sort logs --> write into channel
func (mf *multiFetcher) start() {
	logger.Logger.Info(fmt.Sprintf("start multiFetcher with %d fetcher", len(mf.sortLogs)))
start:
	if len(mf.sortLogs) == 0 {
		mf.close()
		return
	}
	sort.Slice(mf.sortLogs, func(i, j int) bool {
		return mf.sortLogs[i].OccurAt.Before(mf.sortLogs[j].OccurAt)
	})

	mf.wChannel <- mf.sortLogs[0].LogRecordWrapper
	minIdx := mf.sortLogs[0].idx

	for {
		c, ok := <-mf.rChannels[minIdx]
		// channel is closed
		if !ok {
			mf.sortLogs = mf.sortLogs[1:]
			goto start
		}

		if len(mf.sortLogs) > 1 && c.OccurAt.After(mf.sortLogs[1].OccurAt) {
			mf.sortLogs[0] = &indexLog{
				LogRecordWrapper: c,
				idx:              minIdx,
			}
			goto start
		}
		mf.wChannel <- c
	}
}

func (mf *multiFetcher) close() {
	logger.Logger.Info("multiFetcher finish")
	for _, c := range mf.rChannels {
		_, ok := <-c
		if ok {
			close(c)
		}
	}
	close(mf.wChannel)
}
