package helper

import (
	"github.com/WoSai/havok/logger"
	"go.uber.org/zap"
)

var (
	Logger *zap.Logger
)

func init() {
	Logger = logger.Logger
}
