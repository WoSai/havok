package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	iplugin "github.com/wosai/havok/internal/plugin"
	"github.com/wosai/havok/logger"
	pb "github.com/wosai/havok/pkg/genproto"
	"github.com/wosai/havok/pkg/plugin"
	"go.uber.org/zap"
)

type (
	kafkaFetcher struct {
		wg      sync.WaitGroup
		cancel  context.CancelFunc
		merge   mergeService
		clients []*KafkaClient
		begin   time.Time
		end     time.Time
	}

	KafkaClient struct {
		decoder plugin.LogDecoder
		reader  *kafka.Reader
		begin   time.Time
		end     time.Time
		count   int64
		offset  int64
	}

	KafkaOption struct {
		Kafka []*KafkaClientOption `json:"kafka" yaml:"kafka" toml:"kafka"`
		Begin string               `json:"begin" yaml:"begin" toml:"begin"`
		End   string               `json:"end" yaml:"end" toml:"end"`
	}

	KafkaClientOption struct {
		Brokers   []string `json:"brokers" yaml:"brokers" toml:"brokers"`
		Topic     string   `json:"topic" yaml:"topic" toml:"topic"`
		Partition int      `json:"partition" yaml:"partition" toml:"partition"`
		MinBytes  int      `json:"min_bytes" yaml:"min_bytes" toml:"min_bytes"`
		MaxBytes  int      `json:"max_bytes" yaml:"max_bytes" toml:"max_bytes"`
		Offset    int64    `json:"offset" yaml:"offset" toml:"offset"`
	}
)

var _ plugin.Fetcher = (*kafkaFetcher)(nil)

func NewKafkaFetcher(merge mergeService) plugin.Fetcher {
	return &kafkaFetcher{
		merge: merge,
	}
}

// Name Fetcher名称
func (kf *kafkaFetcher) Name() string {
	return "kafka-fetcher"
}

// Apply 传入Fetcher的运行参数
func (kf *kafkaFetcher) Apply(opt any) {
	b, err := json.Marshal(opt)
	if err != nil {
		panic(err)
	}

	var option = new(KafkaOption)

	err = json.Unmarshal(b, option)
	if err != nil {
		panic(err)
	}
	logger.Logger.Info("apply fetcher config", zap.String("name", kf.Name()), zap.Any("config", option))

	kf.begin = ParseTime(option.Begin)
	kf.end = ParseTime(option.End)
	for _, opt := range option.Kafka {
		cli, err := newKafkaClient(opt, kf.begin, kf.end)
		if err != nil {
			panic(err)
		}
		kf.clients = append(kf.clients, cli)
	}
}

// WithDecoder 定义了日志解析对象
func (kf *kafkaFetcher) WithDecoder(decoder plugin.LogDecoder) {
	for _, cli := range kf.clients {
		cli.WithDecoder(decoder)
	}
}

func (kf *kafkaFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	if len(kf.clients) == 0 {
		return errors.New("invalid option: kafka clients is empty")
	}
	subCtx, cancel := context.WithCancel(ctx)
	kf.cancel = cancel

	var kfkError = make(chan error, len(kf.clients))
	var mergeSignal = make(chan error, 1)

	for i, _ := range kf.clients {
		ch := make(chan *pb.LogRecord, 100)
		kf.merge.Merge(ch)

		kf.wg.Add(1)
		go func(ctx context.Context, i int, ch chan *pb.LogRecord) {
			defer kf.wg.Done()
			err := kf.clients[i].Fetch(ctx, ch)
			if err != nil {
				logger.Logger.Error("kafka client Fetch fail", zap.Error(err))
				kfkError <- err
			}
		}(subCtx, i, ch)
	}

	kf.wg.Add(1)
	go func() {
		defer kf.wg.Done()
		err := kf.merge.Output(subCtx, output)
		if err != nil {
			logger.Logger.Error("merge service fail", zap.Error(err))
		}
		mergeSignal <- err
	}()

	select {
	case <-ctx.Done():
		kf.wg.Wait()
		return ctx.Err()
	case err := <-kfkError:
		kf.cancel()
		kf.wg.Wait()
		return err
	case err := <-mergeSignal:
		if err != nil {
			kf.cancel()
			kf.wg.Wait()
			return err
		}
		kf.wg.Wait()
		return nil
	}
}

func newKafkaClient(opt *KafkaClientOption, begin, end time.Time) (*KafkaClient, error) {
	kf := &KafkaClient{}
	kf.begin = begin
	kf.end = end
	kf.offset = opt.Offset

	config := kafka.ReaderConfig{
		Brokers:   opt.Brokers,
		Topic:     opt.Topic,
		Partition: opt.Partition,
		MinBytes:  opt.MinBytes,
		MaxBytes:  opt.MaxBytes,
	}

	err := config.Validate()
	if err != nil {
		return nil, err
	}

	kf.reader = kafka.NewReader(config)
	return kf, err
}

// WithDecoder 定义了日志解析对象
func (kf *KafkaClient) WithDecoder(decoder plugin.LogDecoder) {
	kf.decoder = decoder
}

func (kf *KafkaClient) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	defer close(output)
	if kf.decoder == nil {
		return errors.New("decoder is nil")
	}

	err := kf.reader.SetOffset(kf.offset)
	if err != nil {
		return err
	}
	defer kf.reader.Close()

	for {
		msg, err := kf.reader.ReadMessage(ctx)
		if err != nil {
			logger.Logger.Error("reade kafka message fail", zap.Error(err))
			return err
		}
		log, err := kf.decoder.Decode(msg.Value)
		if err != nil {
			logger.Logger.Error("decode kafka message fail", zap.Error(err))
			continue
		}

		if log.OccurAt.AsTime().After(kf.begin) && log.OccurAt.AsTime().Before(kf.end) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case output <- log:
			}
		}
	}
}

func init() {
	iplugin.Register(NewKafkaFetcher(newMergeService()))
}
