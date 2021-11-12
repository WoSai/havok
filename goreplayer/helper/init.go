package helper

import (
	"github.com/wosai/havok/logger"
	"go.uber.org/zap"
)

var (
	Logger *zap.Logger
)

func init() {
	Logger = logger.Logger
}
