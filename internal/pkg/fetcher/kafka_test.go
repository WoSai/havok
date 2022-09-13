package fetcher

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	pb "github.com/wosai/havok/pkg/genproto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestKafkaFetcher_genBackoff(t *testing.T) {
	kf := &KafkaFetcher{opt: &kafkaOption{End: 99}}
	ok := kf.genBackoff()(&timestamppb.Timestamp{Seconds: 100})
	assert.False(t, ok)
	ok = kf.genBackoff()(&timestamppb.Timestamp{Seconds: 100})
	assert.True(t, ok)
}

func TestKafkaFetcher_Apply(t *testing.T) {
	kf := NewKafkaFetcher()
	kf.Apply(map[string]interface{}{})
}

func TestKafkaReader_ReadMessage(t *testing.T) {
	ctl := gomock.NewController(t)
	reader := NewMockKafkaReader(ctl)
	reader.EXPECT().SetOffset(gomock.Any()).Return(nil)
	reader.EXPECT().ReadMessage(gomock.Any()).Return(Message{Offset: 1}, nil).Times(2)
	reader.EXPECT().ReadMessage(gomock.Any()).Return(Message{}, io.EOF)
	reader.EXPECT().Close()

	ctx := context.Background()

	kf := NewKafkaFetcher()
	kf.withBuiltin(func(option *kafkaOption) (KafkaReader, error) {
		return reader, nil
	})

	kf.Apply(nil)

	kf.WithDecoder(&testDecoder{})

	var count int
	var wg sync.WaitGroup
	var output = make(chan *pb.LogRecord)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range output {
			count++
		}
	}()

	err := kf.Fetch(ctx, output)
	assert.Nil(t, err)
	wg.Wait()
	time.Sleep(time.Second)
	assert.Equal(t, 2, count)
}

func TestKafkaReader_ReadMessage_EOF(t *testing.T) {
	ctl := gomock.NewController(t)
	reader := NewMockKafkaReader(ctl)
	reader.EXPECT().SetOffset(gomock.Any()).Return(nil)
	reader.EXPECT().ReadMessage(gomock.Any()).Return(Message{}, io.EOF)
	reader.EXPECT().Close()

	ctx := context.Background()

	kf := NewKafkaFetcher()
	kf.withBuiltin(func(option *kafkaOption) (KafkaReader, error) {
		return reader, nil
	})

	kf.Apply(nil)

	kf.WithDecoder(&testDecoder{})

	var output = make(chan *pb.LogRecord)
	//go func() {
	//	for log := range output {
	//		fmt.Println(log)
	//	}
	//}()

	err := kf.Fetch(ctx, output)
	assert.Nil(t, err)
}
