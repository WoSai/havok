package fetcher

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	pb "github.com/wosai/havok/pkg/genproto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type (
	testDecoder struct{}
)

func (d *testDecoder) Name() string {
	return "test-decoder"
}

func (d *testDecoder) Decode(b []byte) (*pb.LogRecord, error) {
	return &pb.LogRecord{
		Url:     "http://github.com",
		Method:  "get",
		OccurAt: timestamppb.Now(),
	}, nil
}

func TestSLSFetcher_Apply(t *testing.T) {
	_, err := NewSLSClient(&SLSClientOption{
		Concurrency: 1,
		Endpoint:    "xxx.aliyuncs.com",
	}, time.Now(), time.Now())
	assert.Nil(t, err)
}

func TestSLSFetcher_WithDecoder(t *testing.T) {
	f := &SLSClient{}
	f.WithDecoder(&testDecoder{})
}
