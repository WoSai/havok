package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	aliyunsls "github.com/aliyun/aliyun-log-go-sdk"
	sls "github.com/aliyun/aliyun-log-go-sdk"
	iplugin "github.com/wosai/havok/internal/plugin"
	"github.com/wosai/havok/logger"
	pb "github.com/wosai/havok/pkg/genproto"
	"github.com/wosai/havok/pkg/plugin"
)

type (
	SLSFetcher struct {
		client  aliyunsls.ClientInterface
		store   *aliyunsls.LogStore
		decoder plugin.LogDecoder
		opt     *SLSOption
		begin   time.Time
		end     time.Time
		offset  int64
	}

	SLSOption struct {
		Endpoint        string
		AccessKeyId     string
		AccessKeySecret string
		SecurityToken   string
		ProjectName     string
		StoreName       string
		Topic           string
		Begin           int64
		End             int64
		Query           string
		Concurrency     int64
	}
)

func NewSLSFetcher() plugin.Fetcher {
	return &SLSFetcher{}
}

// Name Fetcher名称
func (sf *SLSFetcher) Name() string {
	return "sls-fetcher"
}

// Apply 传入Fetcher的运行参数
func (sf *SLSFetcher) Apply(opt any) {
	b, err := json.Marshal(opt)
	if err != nil {
		panic(err)
	}

	var option = &SLSOption{Concurrency: 100}

	err = json.Unmarshal(b, option)
	if err != nil {
		panic(err)
	}
	logger.Logger.Info("apply fetcher config", zap.String("name", sf.Name()), zap.Any("config", sf.opt))

	sf.opt = option
	sf.begin = time.Unix(option.Begin, 0)
	sf.end = time.Unix(option.End, 0)
	if sf.opt.Concurrency < 1 {
		panic(sf.Name() + " concurrency must > 0")
	}
}

// WithDecoder 定义了日志解析对象
func (sf *SLSFetcher) WithDecoder(decoder plugin.LogDecoder) {
	sf.decoder = decoder
}

// Fetch 定义了抓取方法，基于日志解析出来的LogRecord要求顺序传出，Fetcher的实现上要判断context.Context是否结束的状态，以主动停止自身的工作
func (sf *SLSFetcher) Fetch(ctx context.Context, output chan<- *pb.LogRecord) error {
	if sf.decoder == nil {
		return errors.New(sf.Name() + " decoder is nil")
	}

	sf.client = sls.CreateNormalInterface(sf.opt.Endpoint, sf.opt.AccessKeyId, sf.opt.AccessKeySecret, sf.opt.SecurityToken)

	var rest = make(chan chan *pb.LogRecord, sf.opt.Concurrency)
	go func() {
		defer close(output)
		for ch := range rest {
			for log := range ch {
				output <- log
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			close(rest)
			return ctx.Err()
		default:
		}
		var ch = make(chan *pb.LogRecord)
		go sf.read(ctx, ch)
		rest <- ch
	}
}

func (sf *SLSFetcher) read(ctx context.Context, ch chan<- *pb.LogRecord) {
	if sf.client == nil {
		return
	}
	var lines int64 = 100
	var retry int = 3
	defer close(ch)

	offset := atomic.AddInt64(&sf.offset, lines) - lines

	for retry > 0 {
		retry--
		logs, err := sf.client.GetLogLines(sf.opt.ProjectName, sf.opt.StoreName, sf.opt.Topic,
			sf.opt.Begin, sf.opt.End, sf.opt.Query, lines, offset, false)
		if err != nil {
			logger.Logger.Error("get sls logs fail", zap.Error(err))
			continue
		}

		// 根据api文档，非Complete表示返回结果不完整，需要重新请求
		if !logs.IsComplete() {
			continue
		}
		// 读完了
		if logs.Count < lines {
			ctx.Done()
		}

		for _, log := range logs.Lines {
			l, err := sf.decoder.Decode(log)
			if err != nil {
				logger.Logger.Error("decode sls log fail", zap.Error(err))
				continue
			}
			if sf.begin.Before(l.OccurAt.AsTime()) && sf.end.After(l.OccurAt.AsTime()) {
				ch <- l
			}
		}
		break
	}
}

func init() {
	iplugin.Register(NewSLSFetcher())
}
