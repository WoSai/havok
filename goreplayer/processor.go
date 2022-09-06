package replayer

import (
	"bytes"
	"context"
	"github.com/wosai/havok/internal/option"
	"github.com/wosai/havok/pkg"
	"go.uber.org/zap"
	"io"
	"net"
	"net/http"
	"net/url"
)

var (
	HavokUserAgent     = "github.com/wosai/havok"
	DefaultHttpHandler pkg.Handler
)

type (
	HandlerFunc func(ctx context.Context, request *pkg.Payload, response pkg.ResponseReader) error

	httpHandler struct {
		client *http.Client
	}

	httpResponseReader struct {
		method string
		url    *url.URL
		header http.Header
		body   []byte
		status int
	}

	ResponseWriter interface {
		With(response *http.Response) error
	}
)

func BuildHttpHandler(opt *option.HttpClientOption) pkg.Handler {
	h := &httpHandler{
		client: buildHttpClient(opt),
	}

	DefaultHttpHandler = h
	return DefaultHttpHandler
}

func (handler *httpHandler) Handle(ctx context.Context, request *pkg.Payload, response pkg.ResponseReader) error {
	// doRequest
	req, err := http.NewRequest(request.Method, request.URL.String(), bytes.NewBuffer(request.Body))
	if err != nil {
		Logger.Error("failed to new request", zap.Error(err))
		return err
	}

	req.Header = request.Header
	if req.Header == nil {
		req.Header = http.Header{}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Add("User-Agent", HavokUserAgent)
	}

	res, err := handler.client.Do(req)
	if err != nil {
		return err
	}
	rw, ok := response.(ResponseWriter)
	if ok {
		err := rw.With(res)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *httpResponseReader) Method() string {
	return r.method
}

func (r *httpResponseReader) URL() *url.URL {
	return r.url
}

func (r *httpResponseReader) Header() http.Header {
	return r.header
}

func (r *httpResponseReader) Read() ([]byte, error) {
	return r.body, nil
}

func (r *httpResponseReader) Status() int {
	return r.status
}

func (r *httpResponseReader) With(response *http.Response) error {
	r.method = response.Request.Method
	r.url = response.Request.URL

	r.header = response.Header
	b, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	r.body = b
	r.status = response.StatusCode
	return nil
}

// chain builds a Handler composed of an inline middleware stack and endpoint
// handler in the order they are passed.
func chain(middlewares []pkg.Middleware, endpoint pkg.Handler) pkg.Handler {
	// Return ahead of time if there aren't any middlewares for the chain
	if len(middlewares) == 0 {
		return endpoint
	}

	// Wrap the end handler with the middleware chain
	h := middlewares[len(middlewares)-1](endpoint)
	for i := len(middlewares) - 2; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
}

func (f HandlerFunc) Handle(ctx context.Context, request *pkg.Payload, response pkg.ResponseReader) error { //  用function实现了interface
	return f(ctx, request, response)
}

func buildHttpClient(opt *option.HttpClientOption) *http.Client {
	return &http.Client{
		Timeout: opt.Timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   opt.Timeout,
				KeepAlive: opt.KeepAliveTimeout,
			}).DialContext,
			DisableKeepAlives:     !opt.KeepAlive,
			MaxIdleConns:          opt.MaxIdleConn,
			MaxIdleConnsPerHost:   opt.MaxIdleConnPerHost,
			IdleConnTimeout:       opt.IdleConnTimeout,
			TLSHandshakeTimeout:   opt.TLSHandshakeTimeout,
			ExpectContinueTimeout: opt.ExpectContinueTimeout,
		},
	}
}
