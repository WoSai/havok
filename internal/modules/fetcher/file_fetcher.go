package fetcher

import (
	"bufio"
	"encoding/json"
	"errors"
	"github.com/wosai/havok/internal/logger"
	"github.com/wosai/havok/internal/option"
	"github.com/wosai/havok/internal/plugin"
	"github.com/wosai/havok/pkg"
	"go.uber.org/zap"
	"os"
)

func init() {
	plugin.DefaultInnerPlugin.Add(&FileFetcher{})
}

type (
	FileFetcher struct {
		path     string
		decoders []pkg.Decoder
		output   chan<- *pkg.LogRecordWrapper
	}

	fileFetcherConfig struct {
		File    string
		Decoder []string
	}
)

func (ff *FileFetcher) InitPlugin(opt *option.PluginOption) error {
	c, ok := opt.Conf.Fetchers["FileFetcher"]
	if !ok {
		return errors.New("not found FileFetcher config")
	}

	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	var conf = &fileFetcherConfig{}
	err = json.Unmarshal(b, conf)
	if err != nil {
		return err
	}
	ff.path = conf.File
	for _, d := range conf.Decoder {
		decoder, ok := plugin.DefaultManager.GetDecoderByName(d)
		if !ok {
			return errors.New("not found decoder: " + d)
		}
		ff.decoders = append(ff.decoders, decoder)
	}
	return nil
}

func (ff *FileFetcher) RegisterPlugin(m pkg.Manager) error {
	m.RegisterFetcher(ff)
	return nil
}

func (ff *FileFetcher) Read(ch chan<- *pkg.LogRecordWrapper) {
	ff.output = ch
	ff.Start()
}

func (ff *FileFetcher) Start() error {

	file, err := os.Open(ff.path)
	if err != nil {
		logger.Logger.Error("failed to open file, stop FileFetcher", zap.Error(err))
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var log *pkg.LogRecordWrapper
		for _, d := range ff.decoders {
			var err error
			log, err = d.Decode(scanner.Bytes())
			if err == nil {
				ff.output <- log
				break
			}
		}
	}

	if err = scanner.Err(); err != nil {
		logger.Logger.Error("failed to load file content, stop FileFetcher", zap.Error(err))
		ff.stop()
		return err
	}
	logger.Logger.Info("finished to fetcher file")
	ff.stop()
	return nil
}

func (ff *FileFetcher) stop() {
	close(ff.output)
}
