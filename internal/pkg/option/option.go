package option

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-jimu/components/config"
	"github.com/go-jimu/components/config/env"
	"github.com/go-jimu/components/config/file"
)

const (
	defaultConfig = "default"
	configDir     = "./configs"
	EnvProfile    = "ENV_PROFILE"
	EnvVarsPrefix = "APP_"
)

func Load(opts ...config.Option) config.Config {
	var options []config.Option
	sources := searchConfigInDir(configDir)
	sources = append(sources, env.NewSource(EnvVarsPrefix))

	options = append(options, config.WithSource(sources...))
	options = append(options, opts...)

	conf := config.New(options...)
	if err := conf.Load(); err != nil {
		panic(err)
	}
	return conf
}

func getConfigFileSuffix() string {
	profile := os.Getenv(EnvProfile)
	if profile == "" {
		return ""
	}
	return "_" + strings.ToLower(profile)
}

func searchConfigInDir(dir string) []config.Source {
	var defaultSource config.Source
	var extends []config.Source

	path, err := filepath.Abs(dir)
	if err != nil {
		panic(err)
	}

	filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		nonExt := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		if nonExt == defaultConfig {
			defaultSource = file.NewSource(path)
			return nil
		}

		suffix := getConfigFileSuffix()
		if strings.HasSuffix(nonExt, suffix) {
			extends = append(extends, file.NewSource(path))
		}
		return nil
	})
	if defaultSource != nil {
		o := []config.Source{defaultSource}
		return append(o, extends...)
	}
	return extends
}
