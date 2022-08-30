package plugin

import (
	"github.com/stretchr/testify/assert"
	"github.com/wosai/havok/internal/option"
	"testing"
)

var cfg = &option.PluginOption{
	Path:     ".",
	Decoders: map[string]interface{}{},
}

func TestBuildLoader(t *testing.T) {

	load := BuildLoader(cfg)
	err := load.Load()
	assert.Nil(t, err)
}
