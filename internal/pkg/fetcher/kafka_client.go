package fetcher

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	kafka2 "github.com/wosai/havok/internal/pkg/fetcher/kafka"
)

type (
	kafkaClient struct {
		reader *kafka.Reader
	}

	Option struct {
		Brokers   []string
		Topic     string
		Partition int
		MinBytes  int
		MaxBytes  int
		MaxWait   time.Duration
	}
)

func NewKafkaClient(option *Option) (kafka2.Reader, error) {
	config := kafka.ReaderConfig{
		Brokers:   option.Brokers,
		Topic:     option.Topic,
		Partition: option.Partition,
		MinBytes:  option.MinBytes,
		MaxBytes:  option.MaxBytes,
	}

	err := config.Validate()
	if err != nil {
		return nil, err
	}

	return &kafkaClient{
		reader: kafka.NewReader(config),
	}, nil
}

func (c *kafkaClient) SetOffset(offset int64) error {
	return c.reader.SetOffset(offset)
}

func (c *kafkaClient) ReadMessage(ctx context.Context) (kafka2.Message, error) {
	msg, err := c.reader.ReadMessage(ctx)
	if err != nil {
		return kafka2.Message{}, err
	}
	var headers = make([]kafka2.Header, len(msg.Headers))
	for i, h := range msg.Headers {
		headers[i] = kafka2.Header{Key: []byte(h.Key), Value: h.Value}
	}
	return kafka2.Message{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       msg.Key,
		Value:     msg.Value,
		Headers:   headers,
		Time:      msg.Time,
	}, nil
}

func (c *kafkaClient) Close() {
	c.reader.Close()
}
