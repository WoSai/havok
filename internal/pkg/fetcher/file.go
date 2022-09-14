package fetcher

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/wosai/havok/internal/plugin"
	pb "github.com/wosai/havok/pkg/genproto"
	iplugin "github.com/wosai/havok/pkg/plugin"
)

type (
	FileFetcher struct {
		filePath string
		decoder  iplugin.LogDecoder
		begin    time.Time
		end      time.Time
	}

	FileOption struct {
		FilePath string
		Begin    int64
		End      int64
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

	ff.filePath = option.FilePath
	ff.begin = time.Unix(option.Begin, 0)
	ff.end = time.Unix(option.End, 0)
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
