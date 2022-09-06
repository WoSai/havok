package dispatcher

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/wosai/havok/pkg"
	"math/rand"
	"testing"
	"time"
)

type (
	testFetcher struct {
		ch     chan<- *pkg.LogRecordWrapper
		cancel context.CancelFunc
	}
)

func newTestFetcher() *testFetcher {
	return &testFetcher{}
}

func (f *testFetcher) Read(ch chan<- *pkg.LogRecordWrapper) {
	f.start(ch)
}

func (f *testFetcher) start(ch chan<- *pkg.LogRecordWrapper) {
	f.ch = ch

	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case f.ch <- &pkg.LogRecordWrapper{
				HashField: "1",
				OccurAt:   time.Now().Add(time.Duration(rand.Intn(10*100)) * time.Millisecond),
			}:
			}
		}
	}()
}

func (f *testFetcher) close() {
	f.cancel()
	time.Sleep(time.Millisecond * 100)
	close(f.ch)
}

func TestNewMultiFetcher_multi(t *testing.T) {
	multi := NewMultiFetcher(newTestFetcher(), newTestFetcher(), newTestFetcher())
	ch := make(chan *pkg.LogRecordWrapper)
	multi.Read(ch)

	for i := 0; i < 10; i++ {
		select {
		case v := <-ch:
			assert.NotNil(t, v)
		}
	}
}

func TestNewMultiFetcher_single(t *testing.T) {
	multi := NewMultiFetcher(newTestFetcher())
	ch := make(chan *pkg.LogRecordWrapper, 5)
	multi.Read(ch)

	for i := 0; i < 10; i++ {
		select {
		case v := <-ch:
			assert.NotNil(t, v)
		}
	}
}

func TestNewMultiFetcher_close(t *testing.T) {
	f1 := newTestFetcher()
	f2 := newTestFetcher()
	multi := NewMultiFetcher(f1, f2)
	ch := make(chan *pkg.LogRecordWrapper)
	multi.Read(ch)

	for i := 0; i < 3; i++ {
		select {
		case v := <-ch:
			assert.NotNil(t, v)
		}
	}

	f1.close()

	for i := 0; i < 3; i++ {
		select {
		case v := <-ch:
			assert.NotNil(t, v)
		}
	}

	f2.close()

	for v := range ch {
		assert.NotNil(t, v)
	}
}
