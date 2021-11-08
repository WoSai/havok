package processor

import (
	"testing"
	"github.com/wosai/havok/types"
	"github.com/wosai/havok/protobuf"
	"github.com/stretchr/testify/assert"
)

func TestExtraProcessor_Act0(t *testing.T) {
	RegisterGlobalActionFunc("extra_send1", func(s *types.Session, key string, data interface{}, params []interface{}) (interface{}, error) {
		return dispatcher.LogRecord{Url: params[0].(string), Method: params[1].(string)}, nil
	})
	var units = []*Unit{
		{
			Key: ".", Actions: []*Actor{
			{"extra_send1", []interface{}{"url1", "method1", "header1", "body1"}},
			{"extra_send1", []interface{}{"url2", "method2", "header2", "body2"}}},
		},
	}
	processor := NewExtraProcssor(units)
	d, err := processor.Act(types.NewSession(), "")
	assert.Nil(t, err)
	assert.Equal(t, len(d.([]map[string][]interface{})), 1)
	ds := d.([]map[string][]interface{})
	assert.Equal(t, len(ds[0]["extra_send1"]), 2)
	assert.Equal(t, ds[0]["extra_send1"][0].(dispatcher.LogRecord).Url, "url1")
	assert.Equal(t, ds[0]["extra_send1"][0].(dispatcher.LogRecord).Method, "method1")
	assert.Equal(t, ds[0]["extra_send1"][1].(dispatcher.LogRecord).Url, "url2")
	assert.Equal(t, ds[0]["extra_send1"][1].(dispatcher.LogRecord).Method, "method2")
}

func TestExtraProcessor_Act1(t *testing.T) {
	RegisterGlobalActionFunc("extra_send1", func(s *types.Session, key string, data interface{}, params []interface{}) (interface{}, error) {
		return dispatcher.LogRecord{Url: params[0].(string), Method: params[1].(string)}, nil
	})
	RegisterGlobalActionFunc("extra_send2", func(s *types.Session, key string, data interface{}, params []interface{}) (interface{}, error) {
		return dispatcher.LogRecord{Url: params[0].(string), Method: params[1].(string)}, nil
	})
	var units = []*Unit{
		{
			Key: ".", Actions: []*Actor{
			{"extra_send1", []interface{}{"url1", "method1", "header1", "body1"}},
			{"extra_send2", []interface{}{"url2", "method2", "header2", "body2"}},
			{"extra_send1", []interface{}{"url3", "method3", "header3", "body3"}}},
		},
	}
	processor := NewExtraProcssor(units)
	d, err := processor.Act(types.NewSession(), "")
	assert.Nil(t, err)
	assert.Equal(t, len(d.([]map[string][]interface{})), 2)
	ds := d.([]map[string][]interface{})
	assert.Equal(t, len(ds[0]["extra_send1"]), 2)
	assert.Equal(t, len(ds[1]["extra_send2"]), 1)
	assert.Equal(t, ds[0]["extra_send1"][0].(dispatcher.LogRecord).Url, "url1")
	assert.Equal(t, ds[0]["extra_send1"][0].(dispatcher.LogRecord).Method, "method1")
	assert.Equal(t, ds[0]["extra_send1"][1].(dispatcher.LogRecord).Url, "url3")
	assert.Equal(t, ds[0]["extra_send1"][1].(dispatcher.LogRecord).Method, "method3")
	assert.Equal(t, ds[1]["extra_send2"][0].(dispatcher.LogRecord).Url, "url2")
	assert.Equal(t, ds[1]["extra_send2"][0].(dispatcher.LogRecord).Method, "method2")
}
