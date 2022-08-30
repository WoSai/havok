package plugin

import (
	"github.com/wosai/havok/internal/option"
	"github.com/wosai/havok/logger"
	"go.uber.org/zap"
	"io/ioutil"
	"path/filepath"
	"plugin"
	"strings"
)

type (
	Loader interface {
		Load() error
		Manager() Manager
	}

	loader struct {
		config  *option.PluginOption
		path    string
		manager *manager
	}
)

func BuildLoader(opt *option.PluginOption) Loader {
	return &loader{
		config: opt,
		path:   opt.Path,
	}
}

func (l *loader) Manager() Manager {
	return l.manager
}

func (l *loader) Load() error {
	c, err := ioutil.ReadDir(l.path)
	if err != nil {
		return err
	}

	l.manager = newPluginManager()

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

func (l *loader) initPlugin(p *plugin.Plugin, m Manager) error {
	iFunc, err := p.Lookup("InitPlugin")
	if err != nil {
		return err
	}

	initFunc := iFunc.(func(opt *option.PluginOption) error)
	if err := initFunc(l.config); err != nil {
		return err
	}

	rFunc, err := p.Lookup("RegisterPlugin")
	if err != nil {
		return err
	}

	registerFunc := rFunc.(func(m Manager) error)
	err = registerFunc(m)
	return err
}
