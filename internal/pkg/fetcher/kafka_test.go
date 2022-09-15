package fetcher

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/wosai/havok/internal/pkg/fetcher/kafka"
	testing2 "github.com/wosai/havok/internal/pkg/fetcher/testing"
	pb "github.com/wosai/havok/pkg/genproto"
)

func TestKafkaReader_ReadMessage(t *testing.T) {
	var (
		ctx  = context.Background()
		msg0 = kafka.Message{
			Offset: 0,
			Key:    []byte("msg-0"),
			Value:  []byte("key-0"),
		}
		msg1 = kafka.Message{
			Offset: 1,
			Key:    []byte("msg-1"),
			Value:  []byte("key-1"),
		}
	)

	type (
		msgAndErr struct {
			kafka.Message
			error
		}
	)

	for _, tc := range []struct {
		name   string
		actual []msgAndErr
		count  []int
	}{
		{
			name:   "error io.EOF",
			actual: []msgAndErr{{kafka.Message{}, io.EOF}},
			count:  []int{0},
		},
		{
			name:   "read finish",
			actual: []msgAndErr{{msg0, nil}, {msg1, nil}, {kafka.Message{}, io.EOF}},
			count:  []int{2},
		},
		{
			name:   "read error will continue",
			actual: []msgAndErr{{msg0, nil}, {msg1, errors.New("read error")}, {kafka.Message{}, io.EOF}},
			count:  []int{1},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			reader := testing2.NewMockKafkaReader(ctl)
			reader.EXPECT().SetOffset(gomock.Any()).Return(nil)
			reader.EXPECT().Close()
			for _, msg := range tc.actual {
				reader.EXPECT().ReadMessage(gomock.Any()).Return(msg.Message, msg.error)
			}

			kf := newTestKafkaFetcher(reader)

			var (
				count  int
				wg     sync.WaitGroup
				output = make(chan *pb.LogRecord)
			)

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
			assert.Equal(t, tc.count[0], count)
		})
	}
}

func newTestKafkaFetcher(reader kafka.Reader) *KafkaFetcher {
	kf := &KafkaFetcher{}
	kf.WithDecoder(&testDecoder{})
	kf.WithBuiltin(func(option *KafkaOption) (kafka.Reader, error) {
		return reader, nil
	})
	kf.Apply(map[string]interface{}{"begin": time.Now().Add(-time.Minute).Unix(), "end": time.Now().Add(time.Minute).Unix()})
	return kf
}
