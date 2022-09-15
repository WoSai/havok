package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/wosai/havok/internal/pkg/fetcher/kafka"
	iplugin "github.com/wosai/havok/internal/plugin"
	"github.com/wosai/havok/logger"
	pb "github.com/wosai/havok/pkg/genproto"
	"github.com/wosai/havok/pkg/plugin"
	"go.uber.org/zap"
)

type (
	KafkaFetcher struct {
		decoder    plugin.LogDecoder
		reader     kafka.Reader
		readerFunc KafkaReaderFunc
		begin      time.Time
		end        time.Time
		threshold  int64
		count      int64
		offset     int64
	}

	KafkaReaderFunc func(option *KafkaOption) (kafka.Reader, error)

	KafkaOption struct {
		Brokers   []string `json:"brokers"`
		Topic     string   `json:"topic"`
		Partition int      `json:"partition"`
		MinBytes  int      `json:"min_bytes"`
		MaxBytes  int      `json:"max_bytes"`
		Offset    int64    `json:"offset"`

		Begin     int64 `json:"begin"`
		End       int64 `json:"end"`
		Threshold int64 `json:"threshold"`
	}

	Backoff func(record *pb.LogRecord) bool
)

var _ plugin.Fetcher = (*KafkaFetcher)(nil)

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

	var option = &KafkaOption{MinBytes: 10e3, MaxBytes: 10e6, Threshold: 100}

	err = json.Unmarshal(b, option)
	if err != nil {
		panic(err)
	}
	logger.Logger.Info("apply fetcher config", zap.String("name", kf.Name()), zap.Any("config", option))

	kf.begin = time.Unix(option.Begin, 0)
	kf.end = time.Unix(option.End, 0)
	kf.threshold = option.Threshold
	kf.offset = option.Offset

	if kf.readerFunc == nil {
		panic("build kafka reader fail")
	}

	r, err := kf.readerFunc(option)
	if err != nil {
		panic(err)
	}
	kf.reader = r
}

func (kf *KafkaFetcher) WithBuiltin(f KafkaReaderFunc) {
	kf.readerFunc = f
}

// WithDecoder 定义了日志解析对象
func (kf *KafkaFetcher) WithDecoder(decoder plugin.LogDecoder) {
	kf.decoder = decoder
}

func (kf *KafkaFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	if kf.reader == nil || kf.decoder == nil {
		return errors.New(kf.Name() + " decoder/reader is nil")
	}

	kf.reader.SetOffset(kf.offset)

	for {
		select {
		case <-ctx.Done():
			break
		default:
		}

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

		if log.OccurAt.AsTime().Before(kf.begin) {
			continue
		}

		if log.OccurAt.AsTime().After(kf.end) {
			break
		}
		output <- log
	}
	kf.reader.Close()
	close(output)
	return nil
}

func init() {
	kf := &KafkaFetcher{}
	kf.WithBuiltin(func(option *KafkaOption) (kafka.Reader, error) {
		return NewKafkaClient(&Option{
			Brokers:   option.Brokers,
			Topic:     option.Topic,
			Partition: option.Partition,
			MinBytes:  option.MinBytes,
			MaxBytes:  option.MaxBytes,
		})
	})
	iplugin.Register(kf)
}
