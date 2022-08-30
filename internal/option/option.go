package option

import (
	"strings"
)

type (
	Env string

	// LoggerOption 日志配置模块
	LoggerOption struct {
		Level      string `default:"info"`
		Name       string
		Filename   string
		MaxSize    int  `default:"100" yaml:"max_size,omitempty" json:"max_size,omitempty"`
		MaxAge     int  `default:"7" yaml:"max_age,omitempty" json:"max_age,omitempty"`
		MaxBackups int  `default:"30" yaml:"max_backups,omitempty" json:"max_backups,omitempty"`
		LocalTime  bool `default:"true" yaml:"local_time,omitempty" json:"local_time,omitempty"`
		Compress   bool
	}

	EventOption struct {
		Mediator struct {
			Concurrency int `default:"20"`
			Buffer      int `default:"100"`
		}
	}

	PluginOption struct {
		Path        string
		Decoders    map[string]interface{}
		Fetchers    map[string]interface{}
		Middlewares map[string]interface{}
	}

	// Option 配置入口
	Option struct {
		Env            Env
		StaticDir      string
		LoginDirectUri string `required:"true"`
		Description    string
		SelfDomain     string `required:"true"`
		Logger         LoggerOption
		RobotUser      []string
		Event          EventOption
		AllowOrigins   []string
		Plugin         PluginOption
	}
)

func (e Env) IsProd() bool {
	if strings.ToLower(string(e)) == "prod" {
		return true
	}
	return false
}

func (e Env) IsTest() bool {
	if strings.ToLower(string(e)) == "test" {
		return true
	}
	return false
}
