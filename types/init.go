package types

import (
	"github.com/wosai/havok/internal/logger"
	"go.uber.org/zap"
)

var (
	// Logger 全局打印日志对象
	Logger *zap.Logger
)

const (
	module = "types"
)

func init() {
	Logger = logger.Logger
}