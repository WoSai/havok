package fetcher

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/wosai/havok/internal/plugin"
	"github.com/wosai/havok/logger"
	pb "github.com/wosai/havok/pkg/genproto"
	iplugin "github.com/wosai/havok/pkg/plugin"
	"go.uber.org/zap"
)

type (
	FileFetcher struct {
		filePath string
		decoder  iplugin.LogDecoder
		begin    time.Time
		end      time.Time
	}

	FileOption struct {
		FilePath string `json:"file_path"`
		Begin    string `json:"begin"`
		End      string `json:"end"`
	}
)

var _ iplugin.Fetcher = (*FileFetcher)(nil)

func NewFileFetcher() iplugin.Fetcher {
	return &FileFetcher{}
}

func (ff *FileFetcher) Name() string {
	return "file-fetcher"
}

// Apply TODO: 重新设计参数
func (ff *FileFetcher) Apply(opt any) {
	b, err := json.Marshal(opt)
	if err != nil {
		panic(err)
	}

	var option = FileOption{}
	err = json.Unmarshal(b, &option)
	if err != nil {
		panic(err)
	}
	logger.Logger.Info("apply fetcher config", zap.String("name", ff.Name()), zap.Any("config", option))

	ff.filePath = option.FilePath
	ff.begin = ParseTime(option.Begin)
	ff.end = ParseTime(option.End)
}

func (ff *FileFetcher) WithDecoder(decoder iplugin.LogDecoder) {
	ff.decoder = decoder
}

func (ff *FileFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	defer close(output)

	file, err := os.Open(ff.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if log, err := ff.decoder.Decode(scanner.Bytes()); err == nil {
			if log.OccurAt.AsTime().Before(ff.begin) {
				continue
			}
			if log.OccurAt.AsTime().After(ff.end) {
				return nil
			}
			output <- log
		}
	}
	return scanner.Err()
}

func init() {
	plugin.Register(NewFileFetcher())
}
