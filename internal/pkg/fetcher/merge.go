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

func (ms *MergeService) Merge(logCh ...<-chan *pb.LogRecord) {
	ms.rChannels = append(ms.rChannels, logCh...)
}

func (ms *MergeService) Output(ctx context.Context, output chan<- *pb.LogRecord) error {
	defer close(output)

	if len(ms.rChannels) == 0 {
		return errors.New("invalid option: rChannels is empty")
	}

	ms.prepareSortLogs()

	for ms.sortLogs.Len() > 0 {
		elem := ms.sortLogs.Front()
		ms.sortLogs.Remove(elem)

		minLog := elem.Value.(*indexLog)
		output <- minLog.LogRecord
		minIdx := minLog.idx

		select {
		case <-ctx.Done():
			return ctx.Err()
		case c, ok := <-ms.rChannels[minIdx]:
			if !ok {
				continue
			}

			log := &indexLog{
				LogRecord: c,
				idx:       minIdx,
			}
			insertSortedLogs(ms.sortLogs, log)
		}
	}
	return nil
}

func (ms *MergeService) prepareSortLogs() {
	var logs = []*indexLog{}
	for i, subCh := range ms.rChannels {
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
		ms.sortLogs.PushBack(log)
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
