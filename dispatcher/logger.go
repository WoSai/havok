package dispatcher

import (
	"github.com/wosai/havok/logger"
	"go.uber.org/zap"
)

var (
	// Logger 全局打印日志对象
	Logger *zap.Logger
)

const (
	module = "dispatcher"
)

func init() {
	Logger = logger.Logger
}
