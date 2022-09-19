package fetcher

import (
	"testing"

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
	f := NewSLSFetcher()
	f.Apply(map[string]interface{}{
		"concurrency": 1,
		"endpoint":    "xxx.aliyuncs.com",
		"begin":       "2006-01-02 15:04:05",
		"end":         "2006-01-02 15:04:05",
	})
}

func TestSLSFetcher_WithDecoder(t *testing.T) {
	f := NewSLSFetcher()
	f.WithDecoder(&testDecoder{})
}
