package replayer

import (
	"context"
	"errors"
	"io"
	"time"

	pb "github.com/wosai/havok/pkg/genproto"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type (
	Inspector struct {
		ID   string
		Host string
		conn *grpc.ClientConn
	}
)

var (
	defaultRate       float32 = 1.0
	DefaultReplayer   *Replayer
	DefaultReplayerId string
)

const (
	DefaultReplayerConcurrency = 3000
)

func NewInspector(host string) (*Inspector, error) {
	ctx, cel := context.WithTimeout(context.Background(), time.Second*5)
	defer cel()
	conn, err := grpc.DialContext(ctx, host, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	return &Inspector{
		ID:   DefaultReplayerId,
		Host: host,
		conn: conn,
	}, nil
}

func (ins *Inspector) Run() error {
	if ins.conn == nil {
		return errors.New("please connect to host first")
	}
	defer ins.conn.Close()
	client := pb.NewHavokClient(ins.conn)
	stream, err := client.Subscribe(context.Background(), &pb.ReplayerRegistration{Id: ins.ID})
	if err != nil {
		return err
	}

	go func(hc pb.HavokClient) {
		for sr := range reportorPipeline {
			hc.Report(context.Background(), sr)
		}
	}(client)

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			Logger.Error("eof", zap.Error(err))
			return err
		}
		if err != nil {
			stream.CloseSend()
			Logger.Error("failed to rec msg", zap.Error(err))
			return err
		}

		switch msg.Type {
		case pb.DispatcherEvent_Subscribed:
			//订阅成功
			Logger.Info("Subscribed successfully, default job configuration", zap.Any("defaultRate", defaultRate))
		case pb.DispatcherEvent_Disconnected:
			Logger.Info("Stop subscribe")
			//订阅失败/中断
		case pb.DispatcherEvent_JobStart:
			// 订阅开始
			jobConfig := msg.GetJob()
			DefaultReplayer.refreshReplayerConfig(jobConfig)
		case pb.DispatcherEvent_JobStop:
			// 订阅结束
			Logger.Info("Job end")
		case pb.DispatcherEvent_JobConfiguration:
			//配置刷新
			jobConfig := msg.GetJob()
			DefaultReplayer.refreshReplayerConfig(jobConfig)
		case pb.DispatcherEvent_LogRecord:
			//日志回放
			replayerPipeline <- msg.GetLog()
		case pb.DispatcherEvent_StatsCollection:
			//统计报告
			submitterPipeline <- msg.GetStats()
		}
	}
}

func RefreshDefaultReplayer(keepAlive bool) *Replayer {
	if DefaultReplayerId == "" {
		DefaultReplayerId = generateReplayerId()
	}
	if DefaultReplayer == nil {
		return NewReplayer(defaultRate, &ProcessorHub{}, DefaultReplayerConcurrency, keepAlive)
	}
	return DefaultReplayer
}
