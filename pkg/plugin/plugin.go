package plugin

import (
	"context"

	pb "github.com/wosai/havok/pkg/genproto"
)

const MagicString = "InitPlugin"

type (
	// Named a named plugin
	Named interface {
		// Name  return plugin name
		Name() string
	}

	// InitFunc 插件初始化函数
	InitFunc func() any

	// Fetcher 定义了日志源的抓取对象
	Fetcher interface {
		Named
		// Apply 传入Fetcher的运行参数
		Apply(any)
		// WithDecoder 定义了日志解析对象
		WithDecoder(LogDecoder)
		// Fetch 定义了抓取方法，基于日志解析出来的LogRecord要求顺序传出，Fetcher的实现上要判断context.Context是否结束的状态，以主动停止自身的工作
		Fetch(context.Context, chan<- *pb.LogRecord) error
	}

	// LogDecoder 日志解析器
	LogDecoder interface {
		Named
		// Decode 解析函数
		Decode([]byte) (*pb.LogRecord, error)
	}

	Valuer func(context.Context, any, string) interface{}

	Processor interface {
		Named
		Store(context.Context, any, string) error
		Set(context.Context, any, string, Valuer) error
		Delete(context.Context, any, string)
	}

	Assertor interface {
		Assert(any) bool
	}
)
