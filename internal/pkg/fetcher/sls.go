package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	iplugin "github.com/wosai/havok/internal/plugin"
	"github.com/wosai/havok/logger"
	pb "github.com/wosai/havok/pkg/genproto"
	"github.com/wosai/havok/pkg/plugin"
)

type (
	SLSFetcher struct {
		client  sls.ClientInterface
		decoder plugin.LogDecoder
		opt     *SLSOption
		begin   time.Time
		end     time.Time
		offset  int64
		readed  int32
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

var lines int64 = 100 // sls GetLogLines 一页最大返回行数为100

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
	defer close(rest)

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
			return ctx.Err()
		default:
		}
		// 读完了, 不再启动新的协程
		if sf.isReaded() {
			break
		}
		var ch = make(chan *pb.LogRecord, lines)
		rest <- ch
		go sf.read(ctx, ch)
	}
	return nil
}

func (sf *SLSFetcher) read(ctx context.Context, ch chan<- *pb.LogRecord) {
	if sf.client == nil || sf.decoder == nil {
		logger.Logger.Error("sls client or decoder is nil")
		return
	}
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
			atomic.CompareAndSwapInt32(&sf.readed, 0, 1)
		}

		for _, line := range logs.Lines {
			log, err := sf.decoder.Decode(line)
			if err != nil {
				logger.Logger.Error("decode sls log fail", zap.Error(err))
				continue
			}
			if sf.begin.Before(log.OccurAt.AsTime()) && sf.end.After(log.OccurAt.AsTime()) {
				ch <- log
			}
		}
		break
	}
}

func (sf *SLSFetcher) isReaded() bool {
	return atomic.LoadInt32(&sf.readed) == 1
}

func init() {
	iplugin.Register(NewSLSFetcher())
}
