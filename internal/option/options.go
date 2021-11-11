package option

type (
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

	InfluxDBOption struct {
		ServerURL     string `default:"http://localhost:8086"`
		AuthToken     string
		BatchSize     int    `default:"20"`
		Measurement   string `default:"report"`
		Organization  string
		Bucket        string
		FlushInterval int `default:"500"` // ms
	}

	Job struct {
		Rate  float32 `required:"true"` // 回放倍数，如2.0，表示一次请求回放两次
		Speed float32 `required:"true"` // 回放速率，如2.0，表示原来10秒内有5次请求发生，回放时会把这些请求在5秒回放完
		Begin int64   `required:"true"`
		End   int64   `required:"true"`
	}

	Fetcher struct {
		Type string `required:"true"`
		File struct {
			Path string
		}
		Sls struct {
			AccessKeyId     string `toml:"access_key_id"`
			AccessKeySecret string `toml:"access_key_secret"`
			Region          string
			Project         string
			Logstore        string
			Expression      string
			Concurrency     int
			PreDownload     int `toml:"pre-download"`
		}
		Kafka struct {
			Brokers []string
			Topic   string
			Offset  int64
		}
	}

	Analyzer struct {
		Name    string `required:"true" default:"base"`
		Handler struct {
			Enable []string
			Plugin []string
		}
	}

	Service struct {
		GRPC string `toml:"grpc" default:":16300"`
		HTTP string `toml:"http" default:":16200"`
	}

	Reporter struct {
		Style struct {
			Name string
		}
		Influxdb InfluxDBOption
	}

	Dispatcher struct {
		Job      Job     `required:"true"`
		Fetcher  Fetcher `required:"true"`
		Analyzer Analyzer
		Service  Service
		Reporter Reporter
	}

	Rule struct {
		Path   string
		Apollo Apollo
	}

	Apollo struct {
		Host      string
		AppID     string
		Namespace string
		Key       string
	}

	Goreplayer struct {
		Host                string `default:"127.0.0.1:16300" required:"true"`
		Selector            string
		KeepAlive           bool
		ReplayerConcurrency int  `required:"true"`
		Rule                Rule `required:"true"`
	}

	// Option 配置入口
	DispatcherOption struct {
		Logger LoggerOption
		Dispatcher
	}

	GoreplayerOption struct {
		Logger LoggerOption
		Goreplayer
	}
)
