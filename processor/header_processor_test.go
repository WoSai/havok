package processor

import (
	"net/http"
	"testing"

	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/wosai/havok/types"
)

var (
	data = "window.variable.push('codeId', '4756935016354312')" +
		".push('browser', 'WEIXIN').push('browserName', '微信').push('memberId', 'E67EFapt2w5zTJD0suZtzMpF').push('merchantId', 'fc16e684-409a-4a38-9a7e-71eb9b283f4a')"

	units = []*Unit{
		{
			Key:     "User-Agent",
			Actions: []*Actor{{"set", []interface{}{"MicroMessage"}}, {"print", nil}},
		},
		{
			Key:     "X-Log-Id",
			Actions: []*Actor{{"store_into_session", []interface{}{"x-log-id"}}},
		},
		{
			Key:     "X-Id",
			Actions: []*Actor{{"get_from_session", []interface{}{"x-log-id"}}},
		},
		{
			Key:     "to-delete",
			Actions: []*Actor{{"delete", nil}},
		},
		{
			Key:     "key-rand-string",
			Actions: []*Actor{{"rand_string", []interface{}{8}}},
		},
		{
			Key:     "key-timestamp",
			Actions: []*Actor{{"current_timestamp", []interface{}{"ms"}}},
		},
		{
			Key:     "key-uuid",
			Actions: []*Actor{{"uuid", nil}},
		},
		{
			Key:     "U-A",
			Actions: []*Actor{{"set", []interface{}{"Alipay1"}}},
		},
		{
			Key:     "U-A",
			Actions: []*Actor{{"add", []interface{}{"Alipay2"}}},
		},
		{
			Key: "Cookies",
			Actions: []*Actor{
				{"set", []interface{}{data}},
				{"regexp_picker", []interface{}{"codeId', '(\\d+)'", int64(1), "@openid"}},
			},
		},
	}
)

func TestNewHTTPHeaderProcessor(t *testing.T) {
	p := NewHTTPHeaderProcessor(nil)
	assert.Nil(t, p.units)
}

func TestHTTPHeaderProcessor_Act(t *testing.T) {
	pro := NewHTTPHeaderProcessor(units)
	header := http.Header{"X-Log-Id": []string{"abc"}}
	se := types.NewSession()
	_, err := pro.Act(se, header)
	assert.Nil(t, err)
	assert.Equal(t, header.Get("User-Agent"), "MicroMessage")
	assert.Equal(t, header.Get("X-Id"), "abc")
	assert.Equal(t, header.Get("to-delete"), "")
	assert.Equal(t, header.Get("Cookies"), data)
	assert.Contains(t, header["U-A"], "Alipay1")
	assert.Contains(t, header["U-A"], "Alipay2")
	assert.Equal(t, len(header.Get("key-rand-string")), 8)
	assert.Equal(t, len(header.Get("key-timestamp")), 13)
	assert.Equal(t, len(header.Get("key-uuid")), 36)

	openId, exits := se.Get("@openid")
	assert.True(t, exits)
	assert.Equal(t, openId, "4756935016354312")
}

func TestRegisterActionFuncInHTTPHeaderProcessor(t *testing.T) {
	RegisterActionFuncInHTTPHeaderProcessor("noname", func(s *types.Session, key string, data interface{}, params []interface{}) (i interface{}, e error) {
		return nil, nil
	})

	us := []*Unit{
		{Key: "something", Actions: []*Actor{{"noname", nil}}},
	}

	pro := NewHTTPHeaderProcessor(us)
	_, err := pro.Act(types.NewSession(), http.Header{})
	assert.Nil(t, err)
}

func TestRegisterAssertFuncInHTTPHeaderProcessor0(t *testing.T) {
	RegisterAssertFuncInHTTPHeaderProcessor("noname", func(s *types.Session, key string, data interface{}, params []interface{}) error {
		return nil
	})

	us := []*Unit{
		{Key: "something", Asserts: []*Assertor{{"noname", nil}}},
	}
	pro := NewHTTPHeaderProcessor(us)
	err := pro.Assert(types.NewSession(), http.Header{})
	assert.Nil(t, err)
}

func TestRegisterAssertFuncInHTTPHeaderProcessor1(t *testing.T) {
	RegisterAssertFuncInHTTPHeaderProcessor("noname", func(s *types.Session, key string, data interface{}, params []interface{}) error {
		return errors.New("make error")
	})

	us := []*Unit{
		{Key: "something", Asserts: []*Assertor{{"noname", nil}}},
	}
	pro := NewHTTPHeaderProcessor(us)
	err := pro.Assert(types.NewSession(), http.Header{})
	assert.Error(t, err)
}
