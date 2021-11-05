package main

import (
	"flag"
	"os"
	"strconv"

	"encoding/json"
	"fmt"
	"github.com/WoSai/havok/apollo"
	"github.com/WoSai/havok/goreplayer"
	"go.uber.org/zap"
)

var (
	version             = "(git commit revision)"
	host                string
	rule                string
	selector            string
	keepAlive           bool
	replayerConcurrency = "REPLAYER_CONCURRENCY"
	processConfig       replayer.ProcessConfig
)

func main() {
	flag.StringVar(&host, "host", "127.0.0.1:16300", "the grpc host address")
	flag.StringVar(&rule, "rule", "rules.json", "rule of processor")
	flag.StringVar(&selector, "selector", "", "api selector plugin")
	flag.BoolVar(&keepAlive, "keepAlive", false, "http client keep alive")
	flag.Parse()

	if data, err := apollo.LoadConfigurationFromApollo(); err == nil {
		replayer.Logger.Info("get configuration from apollo", zap.String("config", data))
		if data == "" {
			replayer.Logger.Panic("empty replayer config!!!")
		}
		err := json.Unmarshal([]byte(data), &processConfig)
		if err != nil {
			replayer.Logger.Panic("failed to serialized to ProcessConfig", zap.String("data", data))
		} else {
			replayer.Logger.Info("succeed to serialized to ProcessConfig")
		}
	} else {
		replayer.Logger.Info("got error when get replayer config from apollo, try to load local config", zap.Error(err))
		pc, err := replayer.NewProcessorConfigFromFile(rule)
		if err != nil {
			replayer.Logger.Panic("failed to parse rule", zap.Error(err))
		} else {
			processConfig = pc
		}
	}
	replayer.Logger.Info("load rules of replayer", zap.Any("rule", processConfig), zap.String("version", version))
	replayer.Logger.Info("current replayer keepAlive status", zap.Bool("status", keepAlive))
	replayer.DefaultReplayer = replayer.RefreshDefaultReplayer(keepAlive)

	ins, err := replayer.NewInspector(host)
	if err != nil {
		replayer.Logger.Panic("failed to connect dispatcher")
	}

	replayer.DefaultReplayer.Selector = replayer.DefaultSelector
	if selector != "" {
		selectorFunc, err := replayer.Load(selector)
		if err != nil {
			replayer.Logger.Panic(fmt.Sprintf("failed to load api selector: %s", selector), zap.Error(err))
			return
		}
		replayer.DefaultReplayer.Selector = selectorFunc
		replayer.Logger.Info(fmt.Sprintf("succeed to load api selector: %s", selector))
	}

	replayer.DefaultReplayer.PH = processConfig.Build()
	if os.Getenv(replayerConcurrency) != "" {
		rc, err := strconv.Atoi(os.Getenv(replayerConcurrency))
		if err != nil {
			replayer.DefaultReplayer.Concurrency = rc
		}
	}

	go replayer.DefaultReplayer.Run()
	replayer.Runner(replayer.DefaultReplayer)
	replayer.Logger.Fatal("replayer down", zap.Error(ins.Run()))
}
