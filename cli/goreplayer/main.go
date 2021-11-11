package main

import (
	"flag"
	"fmt"
	"github.com/wosai/havok/goreplayer"
	"github.com/wosai/havok/internal/logger"
	"github.com/wosai/havok/internal/option"
	_ "go.uber.org/automaxprocs"
	"go.uber.org/zap"
)

var (
	version = "(git commit revision)"
	processConfig replayer.ProcessConfig
	ReplayerOpt   = option.GoreplayerOption{}

	configurationFile            string
	configurationApolloHost      string
	configurationApolloAppID     string
	configurationApolloNameSpace string
	configurationApolloKey       string
)

func main() {
	flag.StringVar(&configurationFile, "config", "./cli", "dispatcher配置文件")
	flag.StringVar(&configurationApolloHost, "apollo_host", "", "apollo host")
	flag.StringVar(&configurationApolloAppID, "apollo_appid", "", "apollo appid")
	flag.StringVar(&configurationApolloNameSpace, "apollo_namespace", "", "apollo namespace")
	flag.StringVar(&configurationApolloKey, "apollo_key", "", "apollo key name")
	flag.Parse()

	if configurationApolloHost != "" {
		option.LoadFromApollo(&ReplayerOpt, configurationApolloHost, configurationApolloAppID, configurationApolloNameSpace, configurationApolloKey)
		logger.BuildLogger(ReplayerOpt.Logger)
		logger.Logger.Info("loaded apollo config", zap.Any("option", ReplayerOpt), zap.String("version", version))
	} else {
		option.LoadFromFile(&ReplayerOpt, configurationFile)
		logger.BuildLogger(ReplayerOpt.Logger)
		logger.Logger.Info("loaded config file", zap.Any("option", ReplayerOpt), zap.String("version", version))
	}
	if ReplayerOpt.Goreplayer.Rule.Apollo.Host != "" {
		var err error
		processConfig, err = replayer.NewProcessorConfigFromApollo(ReplayerOpt.Goreplayer.Rule.Apollo)
		if err != nil {
			logger.Logger.Panic("failed to load rule from apollo", zap.Error(err))
		}
		logger.Logger.Info("load rule from apollo", zap.Any("rule", processConfig))
	} else {
		var err error
		processConfig, err = replayer.NewProcessorConfigFromFile(ReplayerOpt.Goreplayer.Rule.Path)
		if err != nil {
			logger.Logger.Panic("failed to load rule from file", zap.Error(err))
		}
		logger.Logger.Info("load rule from file", zap.Any("rule", processConfig))
	}
	replayer.DefaultReplayer = replayer.RefreshDefaultReplayer(ReplayerOpt.Goreplayer.KeepAlive)

	ins, err := replayer.NewInspector(ReplayerOpt.Goreplayer.Host)
	if err != nil {
		logger.Logger.Panic("failed to connect dispatcher")
	}

	replayer.DefaultReplayer.Selector = replayer.DefaultSelector
	if ReplayerOpt.Goreplayer.Selector != "" {
		selectorFunc, err := replayer.Load(ReplayerOpt.Goreplayer.Selector)
		if err != nil {
			logger.Logger.Panic(fmt.Sprintf("failed to load api selector: %s", ReplayerOpt.Goreplayer.Selector), zap.Error(err))
			return
		}
		replayer.DefaultReplayer.Selector = selectorFunc
		logger.Logger.Info(fmt.Sprintf("succeed to load api selector: %s", ReplayerOpt.Goreplayer.Selector))
	}

	replayer.DefaultReplayer.PH = processConfig.Build()
	replayer.DefaultReplayer.Concurrency = ReplayerOpt.Goreplayer.ReplayerConcurrency

	go replayer.DefaultReplayer.Run()
	replayer.Runner(replayer.DefaultReplayer)
	logger.Logger.Fatal("replayer down", zap.Error(ins.Run()))
}
