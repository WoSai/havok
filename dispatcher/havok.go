package dispatcher

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/wosai/havok/protobuf"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type (
	// Havok 负责grpc服务，以及实际分发日志
	Havok struct {
		replayerManager   *ReplayerManager
		reporter          *Reporter
		channelSize       int
		KeepAliveInterval time.Duration
		Addr              string
		grpcServ          *grpc.Server
		proxy             *ReplayerProxy
		counter           int64
		qps               int64
		//concurrency       chan struct{}
	}

	// HashFunc 可定义日志分发的逻辑
	HashFunc func(string) uint32

	// ReplayerProxy replayer前置代理，用于做日志分发逻辑控制
	ReplayerProxy struct {
		backends map[uint32]string
		hash     HashFunc
		count    uint32
		mu       sync.RWMutex
	}
)

var (
	// ErrDuplicatedReplayer replayer ID冲突
	ErrDuplicatedReplayer = errors.New("duplicated replayer id")
	// ErrReplayerNotExits replayer id不存在
	ErrReplayerNotExits = errors.New("replayer does not exists")
	// ErrEmptyReplayID replayer id 为空
	ErrEmptyReplayID = errors.New("empty replayer id")
	// ErrReplayerHasBeRemoved replayer异常被移除
	ErrReplayerHasBeRemoved = errors.New("replayer has be removed")

	// DefaultHavok Havok单实例对象
	DefaultHavok = NewHavok(nil, nil, 20)

	defaultRoundTrip = &roundtrip{}

	// HavokKeepAliveInterval havok心跳包间隔
	HavokKeepAliveInterval = 1 * time.Minute
	// HavokListenAddr havok内置grpc服务监听地址
	HavokListenAddr = ":16300"
	// HavokSendConcurrency 日志发送worker的最大并发数
	HavokSendConcurrency = 100
)

// NewHavok Havok的构造函数
func NewHavok(rm *ReplayerManager, rep *Reporter, size int) *Havok {
	return &Havok{
		replayerManager:   rm,
		reporter:          rep,
		channelSize:       size,
		KeepAliveInterval: HavokKeepAliveInterval,
		Addr:              HavokListenAddr,
		proxy:             NewReplayerProxy(),
		//concurrency:       make(chan struct{}, HavokSendConcurrency),
	}
}

// Subscribe gRPC接口
func (hv *Havok) Subscribe(reg *pb.ReplayerRegistration, stream pb.Havok_SubscribeServer) error {
	if reg.Id == "" {
		Logger.Error(ErrEmptyReplayID.Error())
		return ErrEmptyReplayID
	}
	replayer, loaded := hv.replayerManager.LoadOrStoreReplayer(reg.Id, NewReplayer(reg.Id, hv.channelSize))
	if loaded {
		Logger.Error("duplicated replayer id: " + reg.Id)
		return ErrDuplicatedReplayer
	}

	hv.proxy.register(reg.Id)

	defer hv.proxy.remove(replayer.ID)
	defer hv.replayerManager.CloseAndRemove(replayer.ID)

	go func(rep *Replayer, msg *pb.DispatcherEvent) { // 防止使用non-buffer channel时造成阻塞
		rep.Send(msg)
	}(replayer, &pb.DispatcherEvent{Type: pb.DispatcherEvent_Subscribed})

	for event := range replayer.Recv() {
		atomic.AddInt64(&hv.counter, 1)
		err := stream.Send(event)
		if err != nil {
			Logger.Error("failed to send event", zap.String("replayer", reg.Id), zap.Error(err))
			return err
		}
		if event.Type == pb.DispatcherEvent_Disconnected {
			Logger.Warn("send disconnection event", zap.String("replayer", reg.Id))
			break
		}
	}
	return io.EOF
}

// Report gRPC接口
func (hv *Havok) Report(ctx context.Context, sr *pb.StatsReport) (*pb.ReportReturn, error) {
	Logger.Info("received report from replayer", zap.String("replayer", sr.ReplayerId), zap.Int32("request_id", sr.RequestId),
		zap.Time("report_at", time.Unix(sr.ReportTime/1e3, (sr.ReportTime%1e3)*1e6)))
	hv.reporter.Collect(sr.ReplayerId, sr.RequestId, sr.PerformanceStats, sr.Stats...)
	return &pb.ReportReturn{RequestId: sr.RequestId}, nil
}

// Deliver 根据replayer id定向投递Event
func (hv *Havok) Deliver(ins string, event *pb.DispatcherEvent) error {
	return hv.replayerManager.Deliver(ins, event)
}

// KeepAlive 心跳机制
func (hv *Havok) KeepAlive() {
	for {
		time.Sleep(hv.KeepAliveInterval)
		hv.replayerManager.Broadcast(&pb.DispatcherEvent{Type: pb.DispatcherEvent_Ping})
	}
}

// Send 日志方法方法，非定向投递
func (hv *Havok) Send(log *LogRecordWrapper) {
	//hv.concurrency <- struct{}{}
	//go func() {
	ins := hv.proxy.Forward(log)
	if ins != "" {
		hv.Deliver(ins,
			&pb.DispatcherEvent{Type: pb.DispatcherEvent_LogRecord, Data: &pb.DispatcherEvent_Log{log.LogRecord}})
	} else {
		Logger.Warn("no inspector is subscribed, current LogRecord would be dropped")
	}
	//	<-hv.concurrency
	//}()

}

// Broadcast 通知所有Inspector对象，可理解为群发
func (hv *Havok) Broadcast(event *pb.DispatcherEvent) {
	if hv.replayerManager != nil {
		hv.replayerManager.Broadcast(event)
	}
}

// DisconnectReplayer 主动移除失效的Replayer对象
func (hv *Havok) DisconnectReplayer(ins string) {
	hv.Deliver(ins, &pb.DispatcherEvent{Type: pb.DispatcherEvent_Disconnected})
	hv.proxy.remove(ins)
}

// Start 主函数，负责监听相关tcp地址
func (hv *Havok) Start() error {
	listener, err := net.Listen("tcp", hv.Addr)
	if err != nil {
		Logger.Error("failed to listen on "+hv.Addr, zap.Error(err))
		return err
	}
	Logger.Info("dispatcher listen on " + listener.Addr().String())
	go hv.KeepAlive()

	go func() {
		var last = hv.counter
		var current int64
		for {
			time.Sleep(1 * time.Second)
			current = atomic.LoadInt64(&hv.counter)
			hv.qps = current - last
			last = current
			Logger.Info("Havok QPS", zap.Int64("havok_qps", hv.qps))
		}
	}()

	hv.grpcServ = grpc.NewServer()
	pb.RegisterHavokServer(hv.grpcServ, hv)
	return hv.grpcServ.Serve(listener)
}

// WithHashFunc 自定义投递的Hash算法，可根据该算法控制投递规则
func (hv *Havok) WithHashFunc(hs HashFunc) *Havok {
	hv.proxy.WithHashFunc(hs)
	return hv
}

func (hv *Havok) WithReplayerManager(rm *ReplayerManager) *Havok {
	if rm != nil {
		hv.replayerManager = rm
	}
	return hv
}

func (hv *Havok) WithReporter(rep *Reporter) *Havok {
	if rep != nil {
		hv.reporter = rep
	}
	return hv
}

func (hv *Havok) Provide() []ProviderMethod {
	return []ProviderMethod{
		{
			Path: "/api/havok/qps",
			Func: func(w http.ResponseWriter, req *http.Request) {
				renderResponse(w, []byte(fmt.Sprintf("{\"code\":200, \"havok_qps\": \"%d\", \"total\": \"%d\"}", hv.qps, atomic.LoadInt64(&hv.counter))), "application/json")
			},
		},
	}
}

// NewReplayerProxy ReplayerProxy的构造函数
func NewReplayerProxy(ins ...string) *ReplayerProxy {
	p := &ReplayerProxy{
		backends: map[uint32]string{},
		hash:     defaultRoundTrip.hash,
	}
	for _, in := range ins {
		p.register(in)
	}
	return p
}

// Forward 根据hash方法选取被投递的Inspector
func (ip *ReplayerProxy) Forward(log *LogRecordWrapper) string {
	if ip.hash == nil {
		ip.mu.Lock()
		ip.hash = defaultRoundTrip.hash
		ip.mu.Unlock()
	}

	ip.mu.RLock()
	defer ip.mu.RUnlock()
	if ip.count > 0 {
		ins := ip.backends[ip.hash(log.HashField)%ip.count]
		return ins
	}
	return ""
}

func (ip *ReplayerProxy) register(ins string) {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	ip.backends[ip.count] = ins
	ip.count++
}

func (ip *ReplayerProxy) remove(id string) {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	Logger.Info("remove replayer from ReplayerProxy", zap.String("replayer", id))
	ip.count--

	backends := map[uint32]string{}
	var index uint32
	for _, ins := range ip.backends {
		if ins == id {
			continue
		} else {
			backends[index] = ins
			index++
		}
	}
	ip.backends = backends
}

// WithHashFunc 更新hash算法
func (ip *ReplayerProxy) WithHashFunc(ha HashFunc) *ReplayerProxy {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	ip.hash = ha
	return ip
}
