package plugin

import (
	"github.com/wosai/havok/dispatcher"
	replayer "github.com/wosai/havok/goreplayer"
	"sync"
)

type (
	Manager interface {
		RegisterFetcher(f dispatcher.Fetcher)
		RegisterMiddleware(m replayer.Middleware)
		RegisterDecoder(name string, d dispatcher.Decoder)
		GetDecoderByName(name string) (dispatcher.Decoder, bool)
	}

	manager struct {
		mux         sync.Mutex // 没必要给每个map单独的锁
		middlewares []replayer.Middleware
		fetchers    []dispatcher.Fetcher
		decoders    map[string]dispatcher.Decoder
	}
)

func newPluginManager() *manager {
	return &manager{}
}

func (pm *manager) RegisterMiddleware(m replayer.Middleware) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.middlewares = append(pm.middlewares, m)
}

func (pm *manager) RegisterDecoder(name string, d dispatcher.Decoder) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.decoders[name] = d
}

func (pm *manager) RegisterFetcher(f dispatcher.Fetcher) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.fetchers = append(pm.fetchers, f)
}

func (pm *manager) GetDecoderByName(name string) (dispatcher.Decoder, bool) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	d, ok := pm.decoders[name]
	return d, ok
}

func (pm *manager) GetFetchers() []dispatcher.Fetcher {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	return pm.fetchers
}

func (pm *manager) GetMiddlewares() []replayer.Middleware {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	return pm.middlewares
}
