package fetcher

import (
	"container/list"
	"context"
	"errors"
	"sort"

	pb "github.com/wosai/havok/pkg/genproto"
)

type (
	mergeService interface {
		Merge(logCh ...<-chan *pb.LogRecord)
		Output(ctx context.Context, output chan<- *pb.LogRecord) error
	}

	MergeService struct {
		cancel    context.CancelFunc
		rChannels []<-chan *pb.LogRecord
		sortLogs  *list.List
	}
)

func newMergeService() mergeService {
	return &MergeService{
		rChannels: []<-chan *pb.LogRecord{},
		sortLogs:  list.New(),
	}
}

func (mf *MergeService) Merge(logCh ...<-chan *pb.LogRecord) {
	mf.rChannels = append(mf.rChannels, logCh...)
}

func (mf *MergeService) Output(ctx context.Context, output chan<- *pb.LogRecord) error {
	defer close(output)

	if len(mf.rChannels) == 0 {
		return errors.New("invalid option: rChannels is empty")
	}

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

func (mf *MergeService) prepareSortLogs() {
	var logs = []*indexLog{}
	for i, subCh := range mf.rChannels {
		if val, ok := <-subCh; ok {
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

func insertSortedLogs(x *list.List, log *indexLog) {
	for e := x.Front(); e != nil; e = e.Next() {
		if log.OccurAt.AsTime().Before(e.Value.(*indexLog).OccurAt.AsTime()) {
			x.InsertBefore(log, e)
			return
		}
	}
	x.PushBack(log)
}
