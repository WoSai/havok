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

func TestBuildLoader(t *testing.T) {

	load := BuildLoader(cfg)
	err := load.Load()
	assert.Nil(t, err)
}
