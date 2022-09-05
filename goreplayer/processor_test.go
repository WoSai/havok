package replayer

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/wosai/havok/internal/option"
	"github.com/wosai/havok/pkg"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

var ctx = context.Background()
var u *url.URL
var conf option.HttpClientOption
var handler pkg.Handler

func setUp() {
	url, err := url.Parse("https://github.com")
	if err != nil {
		panic(err)
	}
	u = url
	conf = option.HttpClientOption{}
	handler = BuildHttpHandler(&conf)
}

func TestBuildHttpHandler(t *testing.T) {
	setUp()
	var resp = &httpResponseReader{}
	err := handler.Handle(
		ctx,
		&pkg.Payload{
			Method: "get",
			URL:    u,
		},
		resp,
	)
	assert.Nil(t, err)
	assert.Equal(t, strings.ToLower(resp.Method()), "get")
	assert.Equal(t, resp.URL().String(), "https://github.com")
	assert.NotNil(t, resp.Header())
	_, err = resp.Read()
	assert.Nil(t, err)
}

func TestChain(t *testing.T) {
	setUp()
	var resp = &httpResponseReader{}
	req := &pkg.Payload{
		Method: "get",
		URL:    u,
		Header: http.Header{},
	}
	mids := []pkg.Middleware{requestLogger(), addFakeHeader()}
	err := chain(mids, handler).Handle(
		ctx,
		req,
		resp,
	)
	assert.Nil(t, err)
	assert.Equal(t, req.Header.Get("fake"), "1")
}

func TestTimerMiddleware(t *testing.T) {
	setUp()
	var resp = &httpResponseReader{}
	req := &pkg.Payload{
		Method: "get",
		URL:    u,
		Header: http.Header{},
	}

	ti := &timer{}
	mids := []pkg.Middleware{TimerMiddleware(ti)}
	err := chain(mids, handler).Handle(
		ctx,
		req,
		resp,
	)
	assert.Nil(t, err)
	assert.Greater(t, ti.Duration(), time.Millisecond)
}

func requestLogger() pkg.Middleware {
	return func(next pkg.Handler) pkg.Handler {
		fn := func(ctx context.Context, request *pkg.Payload, response pkg.ResponseReader) error {
			fmt.Println("log before request", request)
			err := next.Handle(ctx, request, response)
			if err != nil {
				return err
			}
			fmt.Println("log after response", response.URL())
			return nil
		}
		return HandlerFunc(fn)
	}
}

func addFakeHeader() pkg.Middleware {
	return func(next pkg.Handler) pkg.Handler {
		fn := func(ctx context.Context, request *pkg.Payload, response pkg.ResponseReader) error {
			request.Header.Add("fake", "1")
			err := next.Handle(ctx, request, response)
			if err != nil {
				return err
			}
			return nil
		}
		return HandlerFunc(fn)
	}
}
