package plugin

import (
	"context"

	pb "github.com/wosai/havok/pkg/genproto"
)

const MagicString = "InitPlugin"

type (
	// InitFunc 插件初始化函数
	InitFunc func() any

	// Fetcher 定义了日志源的抓取对象
	Fetcher interface {
		// Name Fetcher名称
		Name() string
		// Apply 传入Fetcher的运行参数
		Apply(any)
		// WithDecoder 定义了日志解析对象
		WithDecoder(LogDecoder)
		// Fetch 定义了抓取方法，基于日志解析出来的LogRecord要求顺序传出，Fetcher的实现上要判断context.Context是否结束的状态，以主动停止自身的工作
		Fetch(context.Context, chan<- *pb.LogRecord) error
	}

	// LogDecoder 日志解析器
	LogDecoder interface {
		// Name 插件名称
		Name() string
		// Decode 解析函数
		Decode([]byte) (*pb.LogRecord, error)
	}
)
