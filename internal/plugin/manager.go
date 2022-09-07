package plugin

import (
	"fmt"

	iplugin "github.com/wosai/havok/pkg/plugin"
)

type (
	manager struct {
		plugins map[string]any
	}

	named interface {
		Name() string
	}
)

var pluginManager *manager

func newManager() *manager {
	return &manager{
		plugins: make(map[string]any),
	}
}

func (m *manager) register(plugin any) {
	if plugin == nil {
		panic("nil plugin")
	}

	if n, ok := plugin.(named); ok && n.Name() != "" {
		if _, exists := m.plugins[n.Name()]; exists {
			panic("duplicated name in manager: " + n.Name())
		} else {
			m.plugins[n.Name()] = plugin
		}
	}

	panic("bad plugin")
}

func (m *manager) lookup(name string) any {
	v, ok := m.plugins[name]
	if !ok {
		panic(fmt.Sprintf("plugin named %s does not exists", name))
	}
	return v
}

func Register(plugin any) {
	pluginManager.register(plugin)
}

func LookupFetcher(name string) iplugin.Fetcher {
	if p := pluginManager.lookup(name); p != nil {
		if f, ok := p.(iplugin.Fetcher); ok {
			return f
		}
	}
	panic(fmt.Sprintf("fetched named %s does not exists", name))
}

func LookupDecoder(name string) iplugin.LogDecoder {
	if p := pluginManager.lookup(name); p != nil {
		if d, ok := p.(iplugin.LogDecoder); ok {
			return d
		}
	}
	panic(fmt.Sprintf("decoder named %s does not exists", name))
}

func init() {
	pluginManager = newManager()
}
