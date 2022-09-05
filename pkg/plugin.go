package pkg

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

type (
	LogRecordWrapper struct {
		HashField string
		OccurAt   time.Time
		*LogRecord
	}

	LogRecord struct {
		Url    string
		Method string
		Header map[string]string
		Body   []byte
	}

	Payload struct {
		Name   string // name for api group
		Method string
		URL    *url.URL
		Header http.Header
		Body   []byte
	}

	Fetcher interface {
		Read(chan<- *LogRecordWrapper)
	}

	Decoder interface {
		Decode([]byte) (*LogRecordWrapper, error)
	}

	Handler interface {
		Handle(ctx context.Context, request *Payload, response ResponseReader) error
	}

	Middleware func(next Handler) Handler

	ResponseReader interface {
		Method() string
		URL() *url.URL
		Header() http.Header
		Read() ([]byte, error)
		Status() int
	}

	Manager interface {
		RegisterFetcher(f Fetcher)
		RegisterMiddleware(m Middleware)
		RegisterDecoder(name string, d Decoder)
		GetDecoderByName(name string) (Decoder, bool)
	}
)

type (
	PluginOption struct {
		Decoders    map[string]interface{}
		Fetchers    map[string]interface{}
		Middlewares map[string]interface{}
	}
)
