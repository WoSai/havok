package fetcher

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type (
	kafkaClient struct {
		reader *kafka.Reader
	}
)

func NewKafkaClient(option *kafkaOption) (*kafkaClient, error) {
	config := kafka.ReaderConfig{
		Brokers:   option.Broker,
		Topic:     option.Topic,
		Partition: option.Partition,
		MinBytes:  option.MinBytes,
		MaxBytes:  option.MaxBytes,
		MaxWait:   option.MaxWait,
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

func (c *kafkaClient) ReadMessage(ctx context.Context) (Message, error) {
	msg, err := c.reader.ReadMessage(ctx)
	if err != nil {
		return Message{}, err
	}
	var headers = make([]Header, len(msg.Headers))
	for i, h := range msg.Headers {
		headers[i] = Header{Key: []byte(h.Key), Value: h.Value}
	}
	return Message{
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
