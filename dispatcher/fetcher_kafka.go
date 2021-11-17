package dispatcher

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type (
	PartitionLogRecord struct {
		*LogRecordWrapper
		PID      int
		Previous *PartitionLogRecord
		Next     *PartitionLogRecord
	}

	SortedLogRecords struct {
		gate        *gateMan
		len         int32
		count       int32
		firstRecord *PartitionLogRecord
		mu          sync.RWMutex
	}

	gateMan struct {
		gates  map[int]chan struct{}
		inputs map[int]chan *LogRecordWrapper
		slr    *SortedLogRecords
		count  int32
	}

	GateManV2 struct {
		gates  map[int]chan struct{}
		inputs map[int]chan *LogRecordWrapper
		first  *PartitionLogRecord
		count  int32
		mu     sync.RWMutex
	}

	// KafkaFetcher kafka日志收集者
	KafkaFetcher struct {
		*baseFetcher
		brokers     []string
		topic       string
		wg          sync.WaitGroup
		offsetStart int
		offsetEnd   int
		buffer      map[int]*LogRecordWrapper
		counter     int64
		qps         int64
	}
)

func newGateman() *gateMan {
	return &gateMan{gates: make(map[int]chan struct{}), inputs: make(map[int]chan *LogRecordWrapper)}
}

func (gm *gateMan) register(p int, input chan *LogRecordWrapper) {
	gm.inputs[p] = input
	gm.gates[p] = make(chan struct{}, 1)

	go func(p int, c1 chan struct{}, c2 chan *LogRecordWrapper) {
		atomic.AddInt32(&gm.count, 1)
		for msg := range c2 {
			c1 <- struct{}{}
			gm.slr.Push(&PartitionLogRecord{LogRecordWrapper: msg, PID: p})
		}
		close(c1)
		atomic.AddInt32(&gm.count, -1)
	}(p, gm.gates[p], gm.inputs[p])
}

func (gm *gateMan) open(p int) {
	<-gm.gates[p]
}

func (gm *gateMan) gateCount() int {
	return int(atomic.LoadInt32(&gm.count))
}

func (s *SortedLogRecords) Push(p *PartitionLogRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count++

	// 首次压栈
	if s.firstRecord == nil {
		s.firstRecord = p
		return
	}

	o := s.firstRecord
	if o.OccurAt.After(p.OccurAt) {
		s.firstRecord = p
		p.Next = o
		o.Previous = p
		return
	}

	o = s.firstRecord.Next
	for {
		if o.OccurAt.After(p.OccurAt) {
			o.Previous.Next = p
			p.Previous = o.Previous
			p.Next = o
			o.Previous = p
			return
		}
		if o.Next == nil {
			break
		}
		o = o.Next
	}
	o.Next = p
	p.Previous = o
}

func (s *SortedLogRecords) Pop() (*LogRecordWrapper, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count--

	if s.firstRecord == nil {
		return nil, errors.New("<nil>")
	}

	f := s.firstRecord
	s.firstRecord = f.Next
	s.firstRecord.Previous = nil
	s.gate.open(f.PID)
	return f.LogRecordWrapper, nil
}

func (gm *GateManV2) Register(p int, input chan *LogRecordWrapper) {
	gm.inputs[p] = input
	gm.gates[p] = make(chan struct{}, 1)
	atomic.AddInt32(&gm.count, 1)

	go func(pid int) {
		for msg := range gm.inputs[pid] {
			gm.gates[pid] <- struct{}{}

			// todo:
			gm.Push(&PartitionLogRecord{LogRecordWrapper: msg, PID: pid})
		}
		atomic.AddInt32(&gm.count, -1)
		close(gm.gates[pid])
		delete(gm.inputs, pid)
		delete(gm.gates, pid)
	}(p)
}

func (gm *GateManV2) Push(p *PartitionLogRecord) {

}

func (gm *GateManV2) Pop() *LogRecordWrapper {
	return nil
}

func NewKafkaFetcher(brokers []string, topic string, start, end int) (*KafkaFetcher, error) {
	if len(brokers) == 0 {
		return nil, errors.New("empty broker")
	}

	ps, err := kafka.DefaultDialer.LookupPartitions(context.Background(), "tcp", brokers[0], topic)
	if err != nil {
		return nil, err
	}
	Logger.Info("loop up partitions of topic", zap.String("topic", topic), zap.Int("partition_count", len(ps)), zap.String("broker", brokers[0]))

	fetcher := &KafkaFetcher{
		baseFetcher: newBaseFetcher(),
		brokers:     brokers,
		topic:       topic,
		offsetStart: start,
		offsetEnd:   end,
		buffer:      make(map[int]*LogRecordWrapper),
	}

	for _, partition := range ps {
		fetcher.wg.Add(1)
		go func(p kafka.Partition) {
			defer fetcher.wg.Done()

			reader := kafka.NewReader(kafka.ReaderConfig{
				Brokers:   fetcher.brokers,
				Topic:     fetcher.topic,
				Partition: p.ID,
				MinBytes:  10e3,
				MaxBytes:  10e6,
			})
			reader.SetOffset(kafka.FirstOffset)
			Logger.Info("start to read message from partition", zap.Int("partition", p.ID), zap.String("topic", fetcher.topic))
			for {
				_, err := reader.ReadMessage(context.Background())
				if err != nil {
					Logger.Error("occur error when reading messages from kafka", zap.Error(err), zap.Int("partition", p.ID))
					break
				}
				atomic.AddInt64(&fetcher.counter, 1)
			}
			Logger.Warn("quit for loop")
		}(partition)
	}

	go func() {
		var last = fetcher.counter
		var current int64
		for {
			time.Sleep(1 * time.Second)
			current = atomic.LoadInt64(&fetcher.counter)
			fetcher.qps = current - last
			last = current
			Logger.Info("KafkaFetcher QPS", zap.Int64("fetcher_qps", fetcher.qps), zap.Int64("counter", last))
		}
	}()

	return fetcher, nil
}
