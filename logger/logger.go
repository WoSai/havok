package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

const (
	defaultEnv = "HAVOK_FILE_LOG"
	logLevel   = "HAVOK_LOG_LEVEL"
)

func InitLog(moduleEnv, moduleName string) *zap.Logger {
	conf := zap.NewProductionConfig()
	conf.Sampling = nil
	conf.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	conf.EncoderConfig.TimeKey = "@timestamp"

	level := strings.ToUpper(os.Getenv(logLevel))
	switch level {
	case "DEBUG":
		conf.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "WARN":
		conf.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	default:

	}

	moduleOutput := os.Getenv(moduleEnv)
	defaultOutput := os.Getenv(defaultEnv)
	switch {
	case moduleOutput != "":
		conf.OutputPaths = []string{moduleOutput}
		conf.ErrorOutputPaths = []string{moduleOutput}
	case defaultOutput != "":
		conf.OutputPaths = []string{defaultOutput}
		conf.ErrorOutputPaths = []string{defaultOutput}
	default:
	}

	logger, err := conf.Build()
	if err != nil {
		panic(err)
	}

	if moduleName != "" {
		return logger.Named(moduleName)
	}
	return logger
}

func init() {
	Logger = InitLog("", "havok")
}
