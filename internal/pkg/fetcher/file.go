package fetcher

import (
	"bufio"
	"context"
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
