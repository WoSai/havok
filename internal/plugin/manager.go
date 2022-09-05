package plugin

import (
	"github.com/wosai/havok/pkg"
	"sync"
)

type (
	ManageRepo interface {
		GetFetchers() []pkg.Fetcher
		GetMiddlewares() []pkg.Middleware
	}

	manager struct {
		mux         sync.Mutex // 没必要给每个map单独的锁
		middlewares []pkg.Middleware
		fetchers    []pkg.Fetcher
		decoders    map[string]pkg.Decoder
	}
)

func newPluginManager() *manager {
	return &manager{}
}

func (pm *manager) RegisterMiddleware(m pkg.Middleware) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.middlewares = append(pm.middlewares, m)
}

func (pm *manager) RegisterDecoder(name string, d pkg.Decoder) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.decoders[name] = d
}

func (pm *manager) RegisterFetcher(f pkg.Fetcher) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.fetchers = append(pm.fetchers, f)
}

func (pm *manager) GetDecoderByName(name string) (pkg.Decoder, bool) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	d, ok := pm.decoders[name]
	return d, ok
}

func (pm *manager) GetFetchers() []pkg.Fetcher {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	return pm.fetchers
}

func (pm *manager) GetMiddlewares() []pkg.Middleware {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	return pm.middlewares
}
