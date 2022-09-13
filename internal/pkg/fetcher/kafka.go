package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/wosai/havok/internal/plugin"
	"github.com/wosai/havok/logger"
	pb "github.com/wosai/havok/pkg/genproto"
	iplugin "github.com/wosai/havok/pkg/plugin"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type (
	KafkaFetcher struct {
		decoder iplugin.LogDecoder
		reader  KafkaReader
		builtin func(option *kafkaOption) (KafkaReader, error)
		opt     *kafkaOption
		count   int64
	}

	KafkaReader interface {
		//ApplyConfig(op)
		SetOffset(offset int64) error
		ReadMessage(ctx context.Context) (Message, error)
		Close()
	}

	Message struct {
		Topic     string
		Partition int
		Offset    int64
		Key       []byte
		Value     []byte
		Headers   []Header
		Time      time.Time
	}

	Header struct {
		Key   []byte
		Value []byte
	}

	kafkaOption struct {
		Broker    []string
		Topic     string
		Partition int
		MinBytes  int
		MaxBytes  int
		MaxWait   time.Duration

		Offset    int64
		Begin     int64
		End       int64
		Threshold int64
	}

	Backoff func(timestamp *timestamppb.Timestamp) bool
)

func NewKafkaFetcher() *KafkaFetcher {
	return &KafkaFetcher{}
}

// Name Fetcher名称
func (kf *KafkaFetcher) Name() string {
	return "kafka-fetcher"
}

// Apply 传入Fetcher的运行参数
func (kf *KafkaFetcher) Apply(opt any) {
	b, err := json.Marshal(opt)
	if err != nil {
		panic(err)
	}

	var option = &kafkaOption{MinBytes: 10e3, MaxWait: 10e6, Threshold: 100}

	err = json.Unmarshal(b, option)
	if err != nil {
		panic(err)
	}
	logger.Logger.Info("apply fetcher config", zap.String("name", kf.Name()), zap.Any("config", option))

	kf.opt = option

	if kf.builtin == nil {
		panic("build kafka reader fail")
	}

	r, err := kf.builtin(option)
	if err != nil {
		panic(err)
	}
	kf.reader = r
}

func (kf *KafkaFetcher) withBuiltin(f func(option *kafkaOption) (KafkaReader, error)) {
	kf.builtin = f
}

// WithDecoder 定义了日志解析对象
func (kf *KafkaFetcher) WithDecoder(decoder iplugin.LogDecoder) {
	kf.decoder = decoder
}

func (kf *KafkaFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	if kf.reader == nil || kf.decoder == nil {
		return errors.New(kf.Name() + " decoder/reader is nil")
	}

	kf.reader.SetOffset(kf.opt.Offset)

	for {
		msg, err := kf.reader.ReadMessage(ctx)

		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Logger.Error("get kafka logs fail", zap.Error(err))
			continue
		}
		log, err := kf.decoder.Decode(msg.Value)
		if err != nil {
			logger.Logger.Error("decode kafka log fail", zap.Error(err))
			continue
		}

		// time range [begin:end)
		if kf.opt.Begin <= log.OccurAt.GetSeconds() && kf.opt.End > log.OccurAt.GetSeconds() {
			output <- log
		}

		if kf.genBackoff()(log.OccurAt) {
			break
		}
	}
	kf.reader.Close()
	close(output)
	return nil
}

func (kf *KafkaFetcher) genBackoff() Backoff {
	// 连续100条日志超过时间窗口则认为已经读取结束
	return func(timestamp *timestamppb.Timestamp) bool {
		if timestamp.GetSeconds() >= kf.opt.End {
			kf.count++
			if kf.count >= kf.opt.Threshold {
				return true
			}
		} else {
			kf.count = 0
		}
		return false
	}
}

func init() {
	kf := NewKafkaFetcher()
	kf.withBuiltin(func(option *kafkaOption) (KafkaReader, error) {
		return NewKafkaClient(option)
	})
	plugin.Register(kf)
}
