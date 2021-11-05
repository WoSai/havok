package dispatcher

import (
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/WoSai/havok/protobuf"
	"go.uber.org/zap"
)

type (
	channelStatus int32

	Replayer struct {
		ID         string
		input      chan *pb.DispatcherEvent
		closed     int32
		bufferSize int
	}

	ReplayerManager struct {
		replayers map[string]*Replayer
		mu        sync.RWMutex
	}
)

func NewReplayer(id string, bs int) *Replayer {
	return &Replayer{
		ID:    id,
		input: make(chan *pb.DispatcherEvent, bs),
		//Output:     make(chan *AttackerStatsWrapper, bs),
		bufferSize: bs,
	}
}

func (rep *Replayer) Send(msg *pb.DispatcherEvent) {
	if atomic.LoadInt32(&rep.closed) == 0 {
		rep.input <- msg
		return
	}
	Logger.Error("replayer channel status set to closed", zap.String("replayer", rep.ID), zap.Int32("event", int32(msg.Type)))
}

func (rep *Replayer) Recv() <-chan *pb.DispatcherEvent {
	return rep.input
}

func (rep *Replayer) Close() {
	atomic.StoreInt32(&rep.closed, 1)
}

func (rep *Replayer) CloseChannel() {
	atomic.StoreInt32(&rep.closed, 1)
	time.Sleep(3 * time.Second)
	close(rep.input)
	Logger.Warn("closed replayer channel", zap.String("replayer", rep.ID))
}

func NewReplayerManager() *ReplayerManager {
	return &ReplayerManager{replayers: map[string]*Replayer{}}
}

func (rm *ReplayerManager) LoadOrStoreReplayer(rid string, replayer *Replayer) (*Replayer, bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	val, ok := rm.replayers[rid]
	if !ok {
		rm.replayers[rid] = replayer
		return replayer, false
	}
	return val, ok
}

func (rm *ReplayerManager) ReplaceReplayer(replayer *Replayer) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.replayers[replayer.ID] = replayer
}

func (rm *ReplayerManager) CloseAndRemove(rid string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	Logger.Info("close and remove replayer", zap.String("replayer", rid))
	if val, loaded := rm.replayers[rid]; loaded {
		val.Close()
		Logger.Info("found replayer to delete", zap.String("replayer", rid))
		delete(rm.replayers, rid)
		Logger.Info("deleted replayer", zap.String("replayer", rid))

		go func(c *Replayer) {
			c.CloseChannel()
		}(val)
	}
}

func (rm *ReplayerManager) GetReplayers() []*Replayer {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	var ret []*Replayer
	for _, r := range rm.replayers {
		ret = append(ret, r)
	}
	return ret
}

func (rm *ReplayerManager) Broadcast(de *pb.DispatcherEvent) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	for _, rep := range rm.replayers {
		rep.Send(de)
	}
}

func (rm *ReplayerManager) Deliver(rid string, de *pb.DispatcherEvent) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	val, ok := rm.replayers[rid]
	if !ok {
		Logger.Error("replayer not found", zap.String("replayer", rid))
		return ErrReplayerNotExits
	}
	val.Send(de)
	return nil
}

func (rm *ReplayerManager) Load(rid string) (*Replayer, bool) {
	rm.mu.RLock()
	defer rm.mu.RLock()

	val, loaded := rm.replayers[rid]
	return val, loaded
}
