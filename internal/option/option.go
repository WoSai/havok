package option

import (
	"github.com/wosai/havok/pkg"
	"strings"
	"time"
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

	HttpClientOption struct {
		KeepAlive             bool          `default:"true"`
		Timeout               time.Duration `default:"60s"`
		DialTimeout           time.Duration `default:"30s"`
		KeepAliveTimeout      time.Duration `default:"30s"`
		MaxIdleConn           int           `default:"2000"`
		MaxIdleConnPerHost    int           `default:"1000"`
		IdleConnTimeout       time.Duration `default:"30s"`
		TLSHandshakeTimeout   time.Duration `default:"10s"`
		ExpectContinueTimeout time.Duration `default:"1s"`
	}

	PluginOption struct {
		Path string
		Conf *pkg.PluginOption
	}

	JobOption struct {
		Rate  float32
		Speed float32
		Begin int64
		End   int64
	}

	// Option 配置入口
	Option struct {
		Env         Env
		Description string
		Logger      *LoggerOption
		Plugin      *PluginOption
		HttpClient  *HttpClientOption
		Job         *JobOption
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
