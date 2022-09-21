package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/segmentio/kafka-go"
	iplugin "github.com/wosai/havok/internal/plugin"
	"github.com/wosai/havok/logger"
	pb "github.com/wosai/havok/pkg/genproto"
	"github.com/wosai/havok/pkg/plugin"
	"go.uber.org/zap"
)

type (
	KafkaFetcher struct {
		decoder   plugin.LogDecoder
		reader    *kafka.Reader
		begin     time.Time
		end       time.Time
		threshold int64
		count     int64
		offset    int64
	}

	KafkaReaderFunc func(option *KafkaOption) (kafka.Reader, error)

	KafkaOption struct {
		Brokers   []string `json:"brokers" yaml:"brokers" toml:"brokers"`
		Topic     string   `json:"topic" yaml:"topic" toml:"topic"`
		Partition int      `json:"partition" yaml:"partition" toml:"partition"`
		MinBytes  int      `json:"min_bytes" yaml:"min_bytes" toml:"min_bytes"`
		MaxBytes  int      `json:"max_bytes" yaml:"max_bytes" toml:"max_bytes"`
		Offset    int64    `json:"offset" yaml:"offset" toml:"offset"`

		Begin     string `json:"begin" yaml:"begin" toml:"begin"`
		End       string `json:"end" yaml:"end" toml:"end"`
		Threshold int64  `json:"threshold" yaml:"threshold" toml:"threshold"` // 退出策略的阈值
	}

	Backoff func(record *pb.LogRecord) bool
)

var _ plugin.Fetcher = (*KafkaFetcher)(nil)

func NewKafkaFetcher() plugin.Fetcher {
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

	var option = &KafkaOption{MinBytes: 10e3, MaxBytes: 10e6, Threshold: 100}

	err = json.Unmarshal(b, option)
	if err != nil {
		panic(err)
	}
	logger.Logger.Info("apply fetcher config", zap.String("name", kf.Name()), zap.Any("config", option))

	kf.begin = ParseTime(option.Begin)
	kf.end = ParseTime(option.End)
	kf.threshold = option.Threshold
	kf.offset = option.Offset

	config := kafka.ReaderConfig{
		Brokers:   option.Brokers,
		Topic:     option.Topic,
		Partition: option.Partition,
		MinBytes:  option.MinBytes,
		MaxBytes:  option.MaxBytes,
	}

	err = config.Validate()
	if err != nil {
		panic(err)
	}

	kf.reader = kafka.NewReader(config)
}

// WithDecoder 定义了日志解析对象
func (kf *KafkaFetcher) WithDecoder(decoder plugin.LogDecoder) {
	kf.decoder = decoder
}

func (kf *KafkaFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	defer close(output)
	if kf.decoder == nil {
		return errors.New(kf.Name() + " decoder is nil")
	}

	err := kf.reader.SetOffset(kf.offset)
	if err != nil {
		return err
	}
	defer kf.reader.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := kf.reader.ReadMessage(ctx)
		if err != nil {
			logger.Logger.Error("get kafka logs fail", zap.Error(err))
			continue
		}
		log, err := kf.decoder.Decode(msg.Value)
		if err != nil {
			logger.Logger.Error("decode kafka log fail", zap.Error(err))
			continue
		}

		if log.OccurAt.AsTime().After(kf.begin) && log.OccurAt.AsTime().Before(kf.end) {
			output <- log
		}

		if kf.genBackoff()(log) {
			break
		}
	}
	return nil
}

func (kf *KafkaFetcher) genBackoff() Backoff {
	// 连续 threshold 条日志超过时间窗口则认为已经读取结束
	return func(record *pb.LogRecord) bool {
		if record.OccurAt.AsTime().After(kf.end) {
			kf.count++
			if kf.count >= kf.threshold {
				return true
			}
		} else {
			kf.count = 0
		}
		return false
	}
}

func init() {
	iplugin.Register(NewKafkaFetcher())
}
