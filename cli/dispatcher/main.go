package main

import (
	"errors"
	"flag"
	"github.com/wosai/havok/dispatcher"
	"github.com/wosai/havok/dispatcher/helper"
	"github.com/wosai/havok/internal/logger"
	"github.com/wosai/havok/internal/option"
	pb "github.com/wosai/havok/protobuf"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"
	"net/http"
	"os"
)

//type (
//	dispatcherConfig struct {
//		Job      job
//		Fetcher  fetcher
//		Analyzer analyzer
//		Service  service
//		Reporter reporter
//	}
//
//	job struct {
//		Rate  float32
//		Speed float32
//		Begin int64
//		End   int64
//	}
//
//	fetcher struct {
//		Type string
//		File struct {
//			Path string
//		}
//		Sls struct {
//			AccessKeyId     string `toml:"access_key_id"`
//			AccessKeySecret string `toml:"access_key_secret"`
//			Region          string
//			Project         string
//			Logstore        string
//			Expression      string
//			Concurrency     int
//			PreDownload     int `toml:"pre-download"`
//		}
//		Kafka struct {
//			Brokers []string
//			Topic   string
//			Offset  int64
//		}
//	}
//
//	analyzer struct {
//		Name    string
//		Handler struct{
//			Enable []string
//			Plugin []string
//		}
//	}
//
//	service struct {
//		GRPC string `toml:"grpc"`
//		HTTP string `toml:"http"`
//	}
//
//	reporter struct {
//		Style struct {
//			Name string
//		}
//
//		Influxdb struct {
//			Url      string
//			Database string
//			User     string
//			Password string
//		}
//	}
//)

var (
	configurationFile            string
	configurationApolloHost      string
	configurationApolloAppID     string
	configurationApolloNameSpace string
	configurationApolloKey       string

	version = "(git commit revision)"

	defaultMux *http.ServeMux

	ihelper       *helper.InfluxDBHelper
	DispatcherOpt = &option.DispatcherOption{}
)

func init() {
	flag.StringVar(&configurationFile, "config", "./cli", "dispatcher配置文件")
	flag.StringVar(&configurationApolloHost, "apollo_host", "", "apollo host")
	flag.StringVar(&configurationApolloAppID, "apollo_appid", "", "apollo appid")
	flag.StringVar(&configurationApolloNameSpace, "apollo_namespace", "", "apollo namespace")
	flag.StringVar(&configurationApolloKey, "apollo_key", "", "apollo key name")
}

func handle(mux *http.ServeMux, p dispatcher.Provider) {
	for _, m := range p.Provide() {
		mux.HandleFunc(m.Path, m.Func)
	}
}

func main() {
	flag.Parse()

	if configurationApolloHost != "" {
		option.LoadFromApollo(&DispatcherOpt, configurationApolloHost, configurationApolloAppID, configurationApolloNameSpace, configurationApolloKey)
		logger.BuildLogger(DispatcherOpt.Logger)
		logger.Logger.Info("loaded apollo config", zap.Any("option", DispatcherOpt), zap.String("version", version))
	} else {
		option.LoadFromFile(&DispatcherOpt, configurationFile)
		logger.BuildLogger(DispatcherOpt.Logger)
		logger.Logger.Info("loaded config file", zap.Any("option", DispatcherOpt), zap.String("version", version))
	}

	defaultMux = http.NewServeMux()
	defaultReplayerManager := dispatcher.NewReplayerManager()
	defaultReporter := startReporter(DispatcherOpt.Dispatcher, defaultReplayerManager)

	// job 初始化行为
	go startJob(DispatcherOpt.Dispatcher)

	go func() {
		dispatcher.DefaultHavok.WithHashFunc(dispatcher.DefaultFNVHashPool.Hash).
			WithReplayerManager(defaultReplayerManager).
			WithReporter(defaultReporter) // 自定义havok投递hash函数
		logger.Logger.Error("havok service down", zap.Error(dispatcher.DefaultHavok.Start()))
		os.Exit(1)
	}()

	logger.Logger.Info("http server is listening on port " + DispatcherOpt.Dispatcher.Service.HTTP)
	logger.Logger.Error("dispatcher service was down",
		zap.Error(http.ListenAndServe(DispatcherOpt.Dispatcher.Service.HTTP, defaultMux)))
	os.Exit(1)
}

func startReporter(conf option.Dispatcher, rm *dispatcher.ReplayerManager) *dispatcher.Reporter {
	styleName := conf.Reporter.Style.Name
	var rep *dispatcher.Reporter
	if styleName == "prometheus" {
		//prometheus
		metrics := dispatcher.NewMetrics("havok", dispatcher.HavokAnalyzer, dispatcher.DefaultSelector, dispatcher.ProInput)
		handle(defaultMux, metrics)
		rep = dispatcher.NewReporter(rm, helper.PrintReportToConsole, helper.LogTailFeeder, dispatcher.GrometheusFeed)
	} else if styleName == "influxdb" {
		// reporter初始化行为
		var err error
		ihelper, err = helper.NewInfluxDBHelper(conf.Reporter.Influxdb)
		if err != nil {
			logger.Logger.Error("init influxDB helper failed", zap.Error(err))
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

func startJob(conf option.Dispatcher) {
	jobConf := &pb.JobConfiguration{
		Rate:  conf.Job.Rate,
		Speed: conf.Job.Speed,
		Begin: conf.Job.Begin,
		End:   conf.Job.End,
	}
	job, err := dispatcher.NewJob(jobConf)
	if err != nil {
		logger.Logger.Error("bad job configuration", zap.Error(err))
		os.Exit(1)
	}

	var fetcher dispatcher.Fetcher

	switch conf.Fetcher.Type {
	case "file":
		fetcher = dispatcher.NewFileFetcher(conf.Fetcher.File.Path)

	case "concurrency-sls", "sls":
		fetcher, err = dispatcher.NewAliyunSLSConcurrencyFetcher(conf.Fetcher.Sls.AccessKeyId, conf.Fetcher.Sls.AccessKeySecret,
			conf.Fetcher.Sls.Region, conf.Fetcher.Sls.Project, conf.Fetcher.Sls.Logstore, conf.Fetcher.Sls.Expression,
			conf.Fetcher.Sls.Concurrency, conf.Fetcher.Sls.PreDownload)
		if err != nil {
			panic(err)
		}
		handle(defaultMux, fetcher.(*dispatcher.AliyunSLSConcurrencyFetcher)) // sls接口

	case "kafka-single-partition":
		fetcher, err = dispatcher.NewKafkaSinglePartitionFetcher(conf.Fetcher.Kafka.Brokers, conf.Fetcher.Kafka.Topic, conf.Fetcher.Kafka.Offset)
		if err != nil {
			panic(err)
		}
		handle(defaultMux, fetcher.(*dispatcher.KafkaSinglePartitionFetcher))

	default:
		panic(errors.New("unknown fetcher type"))
	}

	var analyzer dispatcher.Analyzer

	switch conf.Analyzer.Name {
	default:
		pool, err := dispatcher.NewPluginAnalyzerPool(conf.Analyzer.Handler.Plugin...)
		if err != nil {
			logger.Logger.Panic("load Analyzer Plugin fail", zap.Error(err))
			return
		}
		analyzer = dispatcher.NewBaseAnalyzer(pool, conf.Analyzer.Handler.Enable)
	}
	fetcher.WithAnalyzer(analyzer)

	wheel, err := dispatcher.NewTimeWheel(jobConf)
	if err != nil {
		logger.Logger.Error("bad time wheel", zap.Error(err))
		os.Exit(1)
	}
	job.WithTimeWheel(wheel).WithFetcher(fetcher).UseDefaultHavok()

	handle(defaultMux, job)
	handle(defaultMux, dispatcher.DefaultHavok)
}
