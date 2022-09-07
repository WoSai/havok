package plugin

import "fmt"

type (
	LoadOption struct {
		Name    string         `json:"name" yaml:"name" toml:"name"`
		Type    PluginType     `json:"type" yaml:"type" toml:"type"`
		Path    string         `json:"path" yaml:"path" toml:"path"`
		Args    any            `json:"args" yaml:"args" toml:"args"`
		Extends map[string]any `json:"extends,omitempty" yaml:"extends,omitempty" toml:"extends,omitempty"`
	}

	PluginType string
)

const (
	TypeBuiltin PluginType = "builtin"
	TypeLocal   PluginType = "local"
	TypeRemote  PluginType = "remote"
)

// LoadPlugins 暴露该方法即可
func LoadPlugins(opts ...LoadOption) {
	pluginManager = newManager()
	for _, opt := range opts {
		fmt.Println(opt)
	}
}
