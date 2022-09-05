package main

import (
	"flag"
	replayer "github.com/wosai/havok/goreplayer"
	"github.com/wosai/havok/internal/logger"
	"github.com/wosai/havok/internal/option"
	"github.com/wosai/havok/internal/plugin"
	"go.uber.org/zap"
)

var (
	version                    = "(git commit revision)"
	host                       string
	rule                       string
	selector                   string
	keepAlive                  bool
	replayerConcurrency        = "REPLAYER_CONCURRENCY"
	DefaultReplayerConcurrency = 3000
)

func main() {
	conf := option.LoadConfig()
	logger.Logger.Info("configuration", zap.Any("conf", conf))

	l := plugin.BuildLoader(conf.Plugin)
	err := l.Load()
	if err != nil {
		panic(err)
	}

	flag.StringVar(&host, "host", "127.0.0.1:16300", "the grpc host address")
	flag.Parse()

	ins, err := replayer.NewInspector(host)
	if err != nil {
		replayer.Logger.Panic("failed to connect dispatcher")
	}

	httpHandler := replayer.BuildHttpHandler(conf.HttpClient)
	rp := replayer.BuildReplayer(DefaultReplayerConcurrency, httpHandler, plugin.DefaultLoader.ManageRepo().GetMiddlewares())
	go rp.Run()
	replayer.Runner(replayer.DefaultReplayer)
	replayer.Logger.Fatal("replayer down", zap.Error(ins.Run()))
}
