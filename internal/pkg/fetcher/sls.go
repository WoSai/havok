package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
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
		readed  chan struct{}
		wg      sync.WaitGroup
	}

	SLSOption struct {
		Endpoint        string `json:"endpoint" yaml:"endpoint" toml:"fetchers"`
		AccessKeyId     string `json:"access_key_id" yaml:"access_key_id" toml:"access_key_id"`
		AccessKeySecret string `json:"access_key_secret" yaml:"access_key_secret" toml:"access_key_secret"`
		SecurityToken   string `json:"security_token" yaml:"security_token" toml:"security_token"`
		ProjectName     string `json:"project_name" yaml:"project_name" toml:"project_name"`
		StoreName       string `json:"store_name" yaml:"store_name" toml:"store_name"`
		Topic           string `json:"topic" yaml:"topic" toml:"topic"`
		Begin           string `json:"begin" yaml:"begin" toml:"begin"`
		End             string `json:"end" yaml:"end" toml:"end"`
		Query           string `json:"query" yaml:"query" toml:"query"`
		Concurrency     int64  `json:"concurrency" yaml:"concurrency" toml:"concurrency"`
	}
)

var lines int64 = 100 // sls GetLogLines 一页最大返回行数为100

func NewSLSFetcher() plugin.Fetcher {
	return &SLSFetcher{
		readed: make(chan struct{}, 1),
	}
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

	var option = new(SLSOption)

	err = json.Unmarshal(b, option)
	if err != nil {
		panic(err)
	}
	logger.Logger.Info("apply fetcher config", zap.String("name", sf.Name()), zap.Any("config", option))

	sf.opt = option
	sf.begin = ParseTime(option.Begin)
	sf.end = ParseTime(option.End)
	if sf.opt.Concurrency < 1 {
		panic(sf.Name() + " invalid option: " + "concurrency must > 0")
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
	defer sf.wg.Wait()
	defer close(rest)

	sf.wg.Add(1)
	go func() {
		defer sf.wg.Done()
		defer close(output)
		for ch := range rest {
			for log := range ch {
				select {
				case output <- log:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	var offset int64 = 0

	for {
		var ch = make(chan *pb.LogRecord, lines)
		select {
		case <-ctx.Done():
			return ctx.Err()
		// 读完了
		case <-sf.readed:
			return nil
		case rest <- ch:
			sf.wg.Add(1)
			go sf.read(ctx, offset, ch)
			offset = offset + lines
		}
	}
}

func (sf *SLSFetcher) read(ctx context.Context, offset int64, ch chan<- *pb.LogRecord) {
	defer sf.wg.Done()
	defer close(ch)
	if sf.client == nil || sf.decoder == nil {
		logger.Logger.Error("sls client or decoder is nil")
		return
	}

	for retry := 0; retry < 3; retry++ {
		logs, err := sf.client.GetLogLines(sf.opt.ProjectName, sf.opt.StoreName, sf.opt.Topic,
			sf.begin.Unix(), sf.end.Unix(), sf.opt.Query, lines, offset, false)
		if err != nil {
			logger.Logger.Error("get sls logs fail", zap.Error(err))
			continue
		}

		// 根据api文档，非Complete表示返回结果不完整，需要重新请求
		if !logs.IsComplete() {
			continue
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
		// 读完了
		if logs.Count < lines {
			select {
			case sf.readed <- struct{}{}:
			default:
				// don't block
			}
		}
		break
	}
}

func init() {
	iplugin.Register(NewSLSFetcher())
}
