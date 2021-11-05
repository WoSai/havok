package dispatcher

import (
	"errors"
	pb "github.com/WoSai/havok/protobuf"
	"os"
	"time"

	"plugin"
)

type (
	LogRecordBuilder interface {
		Name() string
		Parse([]byte) (hashField string, occurAt time.Time, url string, method string, header map[string]string, body []byte, ok bool)
	}

	logRecordAdapter struct {
		plug LogRecordBuilder
	}
)

func ConvertAnalyzeFunc(log LogRecordBuilder) LogParser {
	return logRecordAdapter{plug: log}
}

func (wap logRecordAdapter) Name() string {
	return wap.plug.Name()
}

func (wap logRecordAdapter) Parse(b []byte) (*LogRecordWrapper, bool) {
	hashField, occurAt, url, method, header, body, ok := wap.plug.Parse(b)
	return &LogRecordWrapper{
		HashField: hashField,
		OccurAt: occurAt,
		LogRecord: &pb.LogRecord{
			Url: url,
			Method: method,
			Header: header,
			Body: body,
		},
	}, ok
}

func Load(path string) (LogParser, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	plug, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}
	an, err := plug.Lookup("LogRecord")
	if err != nil {
		return nil, err
	}
	lr, ok := an.(LogRecordBuilder)
	if !ok {
		return nil, errors.New("plugin not implement interface dispatcher.LogRecordBuilder")
	}
	return ConvertAnalyzeFunc(lr), nil
}
