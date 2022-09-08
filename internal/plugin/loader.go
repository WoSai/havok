package plugin

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	goplugin "plugin"
	"strings"
	"sync/atomic"

	iplugin "github.com/wosai/havok/pkg/plugin"
)

type (
	LoadOption struct {
		Name  string     `json:"name" yaml:"name" toml:"name"`
		Type  PluginType `json:"type" yaml:"type" toml:"type"`
		Path  string     `json:"path" yaml:"path" toml:"path"`
		Args  any        `json:"args" yaml:"args" toml:"args"`
		Needs []string   `json:"needs,omitempty" yaml:"needs,omitempty" toml:"needs,omitempty"`
	}

	PluginType string

	pluginLoader struct {
		name   string
		option LoadOption
		loaded int32
	}
)

const (
	TypeBuiltin PluginType = "builtin"
	TypeLocal   PluginType = "local"
	TypeRemote  PluginType = "remote"
)

// HAVOK_REMOTE_PLUGIN 远程插件下载到此目录，简化处理
const HAVOK_REMOTE_PLUGIN = "~/.havok_remote_plugin"

// LoadPlugins 暴露该方法即可
func LoadPlugins(opts ...LoadOption) {
	loaders := make(map[string]*pluginLoader, len(opts))
	for _, opt := range opts {
		loaders[opt.Name] = &pluginLoader{
			name:   opt.Name,
			option: opt,
			loaded: 0,
		}
	}

	for _, loader := range loaders {
		if !loader.isLoaded() {
			loader.load(loaders)
		}
	}
}

func (l *pluginLoader) isLoaded() bool {
	return atomic.LoadInt32(&l.loaded) == 1
}

func (l *pluginLoader) load(loaders map[string]*pluginLoader) {
	if l.isLoaded() {
		return
	}

	for _, need := range l.option.Needs {
		dep, ok := loaders[need]
		if !ok {
			panic("cannot found dependent plugin to load")
		}
		if !dep.isLoaded() {
			dep.load(loaders)
		}
	}

	defer func() {
		atomic.CompareAndSwapInt32(&l.loaded, 0, 1)
	}()

	switch l.option.Type {
	case TypeBuiltin: // 内置插件要求编译时注入

	case TypeLocal:
		loadLocalSO(l.option.Path)

	case TypeRemote: // TODO: use http.Client to download .so files
		localPath := downloadRemotePlugin(l.option.Path)
		loadLocalSO(localPath)
	}

	applyArgs(l.option)
}

func loadLocalSO(path string) {
	p, err := goplugin.Open(path)
	if err != nil {
		panic(err)
	}
	symbol, err := p.Lookup(iplugin.MagicString)
	if err != nil {
		panic(err)
	}
	fn, ok := symbol.(iplugin.InitFunc)
	if !ok {
		panic("invalid init function in plugin")
	}
	instance := fn()
	pluginManager.register(instance)
}

func applyArgs(opt LoadOption) {
	plugin := pluginManager.lookup(opt.Name)
	if plugin == nil {
		panic(fmt.Errorf("bad plugin: %s", opt.Name))
	}

	switch v := plugin.(type) {
	case iplugin.Fetcher:
		v.Apply(opt.Args)
		if len(opt.Needs) == 1 {
			v.WithDecoder(LookupDecoder(opt.Needs[0]))
		} else if len(opt.Needs) > 1 {
			panic("invalid plugin option: fetcher.needs > 1")
		}
		return

	case iplugin.LogDecoder: // TODO:
	}
}

func downloadRemotePlugin(remotePath string) string {
	u, err := url.Parse(remotePath)
	if err != nil {
		panic(err)
	}
	segments := strings.Split(u.Path, "/")
	fileName := segments[len(segments)-1]

	d, err := filepath.Abs(HAVOK_REMOTE_PLUGIN)
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(d); os.IsNotExist(err) {
		if err = os.Mkdir(d, os.ModeDir); err != nil {
			panic(err)
		}
	}
	fp := filepath.Join(d, fileName)

	file, err := os.Create(fp)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			req.URL.Opaque = req.URL.Path
			return nil
		},
	}
	res, err := client.Get(remotePath)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	if _, err = io.Copy(file, res.Body); err != nil {
		panic(err)
	}
	return fp
}
