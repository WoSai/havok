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
	pb "github.com/wosai/havok/pkg/genproto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestKafkaFetcher_genBackoff(t *testing.T) {
	var endSecond int64 = 99
	kf := &KafkaFetcher{opt: &kafkaOption{End: endSecond, Threshold: 2}}

	for _, tc := range []struct {
		name     string
		second   int64
		expected bool
	}{
		{
			name:     "log before end, less than threshold",
			second:   endSecond - 1,
			expected: false,
		},
		{
			name:     "log after end, less than threshold",
			second:   endSecond,
			expected: false,
		},
		{
			name:     "log after end, more than threshold",
			second:   endSecond,
			expected: true,
		},
		{
			name:     "log before end, more than threshold, reset threshold",
			second:   endSecond - 1,
			expected: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok := kf.genBackoff()(&pb.LogRecord{OccurAt: &timestamppb.Timestamp{Seconds: tc.second}})
			assert.Equal(t, tc.expected, ok)
		})
	}
}

func TestKafkaReader_ReadMessage(t *testing.T) {
	var (
		ctx  = context.Background()
		msg0 = Message{
			Offset: 0,
			Key:    []byte("msg-0"),
			Value:  []byte("key-0"),
		}
		msg1 = Message{
			Offset: 1,
			Key:    []byte("msg-1"),
			Value:  []byte("key-1"),
		}
	)

	type (
		msgAndErr struct {
			Message
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
			actual: []msgAndErr{{Message{}, io.EOF}},
			count:  []int{0},
		},
		{
			name:   "read finish",
			actual: []msgAndErr{{msg0, nil}, {msg1, nil}, {Message{}, io.EOF}},
			count:  []int{2},
		},
		{
			name:   "read error will continue",
			actual: []msgAndErr{{msg0, nil}, {msg1, errors.New("")}, {Message{}, io.EOF}},
			count:  []int{1},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctl := gomock.NewController(t)
			reader := NewMockKafkaReader(ctl)
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

func newTestKafkaFetcher(reader KafkaReader) *KafkaFetcher {
	kf := NewKafkaFetcher()
	kf.WithDecoder(&testDecoder{})
	kf.withBuiltin(func(option *kafkaOption) (KafkaReader, error) {
		return reader, nil
	})
	kf.Apply(map[string]interface{}{"begin": time.Now().Add(-time.Minute).Unix(), "end": time.Now().Add(time.Minute).Unix()})
	return kf
}
