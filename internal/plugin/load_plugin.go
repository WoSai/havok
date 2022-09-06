package plugin

import (
	"github.com/wosai/havok/internal/logger"
	"github.com/wosai/havok/internal/option"
	"github.com/wosai/havok/pkg"
	"go.uber.org/zap"
	"io/ioutil"
	"path/filepath"
	"plugin"
	"strings"
)

var DefaultLoader Loader = (*multiLoader)(nil)
var DefaultManager *manager = newPluginManager()
var DefaultInnerPlugin = newInnerLoader()

type (
	Loader interface {
		Load() error
		ManageRepo() ManageRepo
	}

	multiLoader struct {
		load       []Loader
		manageRepo []ManageRepo
	}

	// go_plugin加载器
	loader struct {
		config  *option.PluginOption
		path    string
		manager *manager
	}

	// 内部加载器
	innerLoader struct {
		config  *option.PluginOption
		plugin  []innerPlugin
		manager *manager
	}

	innerPlugin interface {
		InitPlugin(opt *option.PluginOption) error
		RegisterPlugin(m pkg.Manager) error
	}
)

func BuildLoader(opt *option.PluginOption) Loader {
	DefaultInnerPlugin.config = opt
	l := &multiLoader{load: []Loader{newLoader(opt), DefaultInnerPlugin}}
	DefaultLoader = l
	return l
}

func (l *multiLoader) Load() error {
	for _, load := range l.load {
		err := load.Load()
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *multiLoader) ManageRepo() ManageRepo {
	return DefaultManager
}

func newLoader(opt *option.PluginOption) *loader {
	return &loader{
		config:  opt,
		path:    opt.Path,
		manager: DefaultManager,
	}
}

func (l *loader) ManageRepo() ManageRepo {
	return l.manager
}

func (l *loader) Load() error {
	c, err := ioutil.ReadDir(l.path)
	if err != nil {
		return err
	}

	for _, entry := range c {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".so") {
			fullpath := filepath.Join(l.path, entry.Name())
			logger.Logger.Info("found plugin file", zap.String("path", fullpath))

			p, err := plugin.Open(fullpath)
			if err != nil {
				return err
			}

			err = l.initPlugin(p, l.manager)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *loader) initPlugin(p *plugin.Plugin, m pkg.Manager) error {
	iFunc, err := p.Lookup("InitPlugin")
	if err != nil {
		return err
	}

	initFunc := iFunc.(func(opt *pkg.PluginOption) error)
	if err := initFunc(l.config.Conf); err != nil {
		return err
	}

	rFunc, err := p.Lookup("RegisterPlugin")
	if err != nil {
		return err
	}

	registerFunc := rFunc.(func(m pkg.Manager) error)
	err = registerFunc(m)
	return err
}

func newInnerLoader() *innerLoader {
	return &innerLoader{
		plugin:  []innerPlugin{},
		manager: DefaultManager,
	}
}

func (in *innerLoader) Add(p ...innerPlugin) {
	in.plugin = append(in.plugin, p...)
}

func (in *innerLoader) Load() error {
	for _, p := range in.plugin {
		err := p.InitPlugin(in.config)
		if err != nil {
			return err
		}
		err = p.RegisterPlugin(in.manager)
		if err != nil {
			return err
		}
	}
	return nil
}

func (in *innerLoader) ManageRepo() ManageRepo {
	return in.manager
}
