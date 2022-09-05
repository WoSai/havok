package replayer

import (
	"context"
	"github.com/wosai/havok/pkg"
	"time"
)

type (
	// Timer 计时器
	Timer interface {
		Start()
		End()
		Duration() time.Duration
	}

	timer struct {
		start    time.Time
		duration time.Duration
	}
)

func (t *timer) Start() {
	t.start = time.Now()
}

func (t *timer) End() {
	t.duration = time.Since(t.start)
}

func (t *timer) Duration() time.Duration {
	return t.duration
}

func TimerMiddleware(t Timer) pkg.Middleware {
	return func(next pkg.Handler) pkg.Handler {
		fn := func(ctx context.Context, request *pkg.Payload, response pkg.ResponseReader) error {
			t.Start()
			err := next.Handle(ctx, request, response)
			t.End()
			return err
		}
		return HandlerFunc(fn)
	}
}
