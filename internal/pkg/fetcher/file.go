package fetcher

import (
	"bufio"
	"context"
	"os"
	"time"

	pb "github.com/wosai/havok/pkg/genproto"
	"github.com/wosai/havok/pkg/plugin"
)

type (
	FileFetcher struct {
		filePath string
		decoder  plugin.LogDecoder
		begin    time.Time
		end      time.Time
	}
)

var _ plugin.Fetcher = (*FileFetcher)(nil)

func NewFileFetcher() plugin.Fetcher {
	return &FileFetcher{}
}

func (ff *FileFetcher) Name() string {
	return "file-fetcher"
}

// Apply TODO: 重新设计参数
func (ff *FileFetcher) Apply(opt any) {

}

func (ff *FileFetcher) WithDecoder(decoder plugin.LogDecoder) {
	ff.decoder = decoder
}

func (ff *FileFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
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
