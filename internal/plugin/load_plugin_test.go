package plugin

import (
	"github.com/stretchr/testify/assert"
	"github.com/wosai/havok/internal/option"
	"github.com/wosai/havok/pkg"
	"testing"
)

var cfg = &option.PluginOption{
	Path: ".",
	Conf: &pkg.PluginOption{},
}

var inner = newInnerLoader()

type (
	fakeInnerLoader struct{}
)

func (l *fakeInnerLoader) InitPlugin(opt *option.PluginOption) error {
	return nil
}

func (l *fakeInnerLoader) RegisterPlugin(m pkg.Manager) error {
	return nil
}

func TestBuildLoader(t *testing.T) {

	load := BuildLoader(cfg)
	err := load.Load()
	assert.Nil(t, err)
}

func TestInnerLoader_Add(t *testing.T) {
	inner.Add(&fakeInnerLoader{})
	err := inner.Load()
	assert.Nil(t, err)
}
