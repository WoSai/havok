package plugin

import (
	"github.com/stretchr/testify/assert"
	"github.com/wosai/havok/pkg"
	"testing"
	"time"
)

var testManger = newPluginManager()

type (
	testDecoder struct{}

	testFetcher struct{}
)

func (d *testDecoder) Decode(b []byte) (*pkg.LogRecordWrapper, error) {
	return &pkg.LogRecordWrapper{
		HashField: "1",
		OccurAt:   time.Now(),
		LogRecord: &pkg.LogRecord{
			Url:    "http://github.com",
			Method: "get",
		},
	}, nil
}

func (f *testFetcher) Read(chan<- *pkg.LogRecordWrapper) {

}

func TestManager_RegisterDecoder(t *testing.T) {
	testManger.RegisterDecoder("decoder", &testDecoder{})
	_, ok := testManger.GetDecoderByName("decoder")
	assert.True(t, ok)
}

func TestManager_RegisterFetcher(t *testing.T) {
	testManger.RegisterFetcher(&testFetcher{})
	assert.Len(t, testManger.GetFetchers(), 1)
}

func TestManager_RegisterMiddleware(t *testing.T) {
	testManger.RegisterMiddleware(fakeMiddleware)
	assert.Len(t, testManger.GetMiddlewares(), 1)
}

func fakeMiddleware(next pkg.Handler) pkg.Handler {
	return next
}


