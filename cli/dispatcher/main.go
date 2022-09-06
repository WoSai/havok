package main

import (
	"errors"
	"flag"
	"github.com/wosai/havok/dispatcher"
	"github.com/wosai/havok/internal/option"
	"github.com/wosai/havok/internal/plugin"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
	"github.com/wosai/havok/apollo"
	"github.com/wosai/havok/dispatcher/helper"
	_ "github.com/wosai/havok/internal/modules"
	pb "github.com/wosai/havok/protobuf"
	"go.uber.org/zap"
)

type (
	dispatcherConfig struct {
		Job      *option.JobOption
		Fetcher  fetcher
		Analyzer analyzer
		Service  service
		Reporter reporter
	}

	fetcher struct {
		Type string
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

	analyzer struct {
		Name    string
		Handler []string
	}

	service struct {
		GRPC string `toml:"grpc"`
		HTTP string `toml:"http"`
	}

	reporter struct {
		Style struct {
			Name string
		}

		Influxdb struct {
			Url      string
			Database string
			User     string
			Password string
		}
	}
)

var (
	configurationFile string
	version           = "(git commit revision)"

	defaultMux *http.ServeMux

	reporterInfluxdbURL      = "REPORTER_INFLUXDB_URL"
	reporterInlfuxdbDatabase = "REPORTER_INFLUXDB_DATABASE"
	reporterInfluxdbUser     = "REPORTER_INFLUXDB_USER"
	reporterInfluxdbPassword = "REPORTER_INFLUXDB_PASSWORD"
)

func init() {
	flag.StringVar(&configurationFile, "config", "", "dispatcher配置文件")
}

func currentFilePath() string {
	_, filename, _, _ := runtime.Caller(1)
	return filename
}

func handle(mux *http.ServeMux, p dispatcher.Provider) {
	for _, m := range p.Provide() {
		mux.HandleFunc(m.Path, m.Func)
	}
}

func main() {
	flag.Parse()

	// 加载配置文件
	var conf dispatcherConfig
	if ca, err := apollo.LoadConfigurationFromApollo(); err == nil {
		dispatcher.Logger.Info("get configuration from apollo", zap.String("config", ca))
		if ca == "" {
			dispatcher.Logger.Panic("empty dispatcher config!!!")
		}
		_, err := toml.Decode(ca, &conf)
		if err != nil {
			dispatcher.Logger.Panic("failed to decode configuration from apollo", zap.Error(err))
		}
	} else {
		dispatcher.Logger.Info("got error when get dispatcher config from apollo, try to load local config", zap.Error(err))
		if configurationFile == "" {
			configurationFile = filepath.Join(filepath.Dir(currentFilePath()), "dispatcher.toml")
		}
		if _, err := toml.DecodeFile(configurationFile, &conf); err != nil {
			dispatcher.Logger.Panic("failed to decode file", zap.Error(err))
		}
	}
	dispatcher.Logger.Info("loaded configurations", zap.Any("config", conf), zap.String("version", version))

	defaultMux = http.NewServeMux()
	defaultReplayerManager := dispatcher.NewReplayerManager()
	defaultReporter := startReporter(conf, defaultReplayerManager)

	opt := option.LoadConfig()
	dispatcher.Logger.Info("configuration", zap.Any("conf", opt))

	plugin.BuildLoader(opt.Plugin)

	// job 初始化行为
	go startJob(conf.Job)

	go func() {
		dispatcher.DefaultHavok.WithHashFunc(dispatcher.DefaultFNVHashPool.Hash).
			WithReplayerManager(defaultReplayerManager).
			WithReporter(defaultReporter) // 自定义havok投递hash函数
		dispatcher.Logger.Error("havok service down", zap.Error(dispatcher.DefaultHavok.Start()))
		os.Exit(1)
	}()

	dispatcher.Logger.Info("http server is listening on port " + conf.Service.HTTP)
	dispatcher.Logger.Error("dispatcher service was down",
		zap.Error(http.ListenAndServe(conf.Service.HTTP, defaultMux)))
	os.Exit(1)
}

func startReporter(conf dispatcherConfig, rm *dispatcher.ReplayerManager) *dispatcher.Reporter {
	styleName := conf.Reporter.Style.Name
	var rep *dispatcher.Reporter
	if styleName == "prometheus" {
		//prometheus
		metrics := dispatcher.NewMetrics("havok", dispatcher.HavokAnalyzer, dispatcher.DefaultSelector, dispatcher.ProInput)
		handle(defaultMux, metrics)
		rep = dispatcher.NewReporter(rm, helper.PrintReportToConsole, helper.LogTailFeeder, dispatcher.GrometheusFeed)
	} else if styleName == "influxdb" {
		// reporter初始化行为
		ic := helper.NewInfluxDBHelperConfig()

		switch {
		case os.Getenv(reporterInfluxdbURL) != "":
			ic.URL = os.Getenv(reporterInfluxdbURL)
		case conf.Reporter.Influxdb.Url != "":
			ic.URL = conf.Reporter.Influxdb.Url
		}

		switch {
		case os.Getenv(reporterInlfuxdbDatabase) != "":
			ic.Database = os.Getenv(reporterInlfuxdbDatabase)
		case conf.Reporter.Influxdb.Database != "":
			ic.Database = conf.Reporter.Influxdb.Database
		}

		switch {
		case os.Getenv(reporterInfluxdbUser) != "":
			ic.User = os.Getenv(reporterInfluxdbUser)
		case conf.Reporter.Influxdb.User != "":
			ic.User = conf.Reporter.Influxdb.User
		}

		switch {
		case os.Getenv(reporterInfluxdbPassword) != "":
			ic.User = os.Getenv(reporterInfluxdbPassword)
		case conf.Reporter.Influxdb.Password != "":
			ic.User = conf.Reporter.Influxdb.Password
		}

		ihelper, err := helper.NewInfluxDBHelper(ic)
		if err != nil {
			dispatcher.Logger.Error("init influxDB helper failed", zap.Error(err))
			panic(err)
		}

		rep = dispatcher.NewReporter(rm, helper.PrintReportToConsole, helper.LogTailFeeder, ihelper.HandleReport())
	} else {
		panic(errors.New("unknown report style"))
	}

	go rep.PeriodicRequest()
	go rep.Run()
	handle(defaultMux, rep)
	return rep
}

func startJob(conf *option.JobOption) {
	err := plugin.DefaultLoader.Load()
	if err != nil {
		panic(err)
	}
	multi := dispatcher.NewMultiFetcher(plugin.DefaultLoader.ManageRepo().GetFetchers()...)

	jobConf := &pb.JobConfiguration{
		Rate:  conf.Rate,
		Speed: conf.Speed,
		Begin: conf.Begin,
		End:   conf.End,
	}
	job, err := dispatcher.NewJob(jobConf)
	if err != nil {
		dispatcher.Logger.Error("bad job configuration", zap.Error(err))
		panic(err)
	}

	wheel, err := dispatcher.NewTimeWheel(jobConf)
	if err != nil {
		dispatcher.Logger.Error("bad time wheel", zap.Error(err))
		panic(err)
	}
	job.WithTimeWheel(wheel).WithFetcher(multi).UseDefaultHavok()

	handle(defaultMux, job)
	handle(defaultMux, dispatcher.DefaultHavok)
}
