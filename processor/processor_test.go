package processor

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/WoSai/havok/types"
	"net/url"
	"errors"
)

func TestRandStringBytesMaskImprSrc0(t *testing.T) {
	a := RandStringBytesMaskImprSrc(0)
	assert.Empty(t, a)
}

func TestRandStringBytesMaskImprSrc1(t *testing.T) {
	a := RandStringBytesMaskImprSrc(10)
	assert.Len(t, a, 10)
}

func TestFormBodyProcessor_ActPrint(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActSet0(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"set", []interface{}{"ok"}}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActSet1(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"set", []interface{}{1}}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Error(t, err)
}

func TestFormBodyProcessor_ActSet2(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "four", Actions: []*Actor{{"set", []interface{}{"ok"}}},
		},
		{
			Key: "four", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActReverse(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"reverse", nil}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActAdd0(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "four", Actions: []*Actor{{"add", []interface{}{"ok"}}},
		},
		{
			Key: "four", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActAdd1(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"add", []interface{}{"ok"}}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActDelete0(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"delete", nil}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActDelete1(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "four", Actions: []*Actor{{"delete", nil}},
		},
		{
			Key: "four", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActGetFromSession0(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"get_from_session", []interface{}{"cot-one"}}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	s := types.NewSession()
	s.Put("cot-one", "ok")
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(s, query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActGetFromSession1(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "four", Actions: []*Actor{{"get_from_session", []interface{}{"cot-one"}}},
		},
		{
			Key: "four", Actions: []*Actor{{"print", nil}},
		},
	}
	s := types.NewSession()
	s.Put("cot-one", "ok")
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(s, query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActStoreSession0(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"store_into_session", []interface{}{"cot-one"}}},
		},
	}
	s := types.NewSession()
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(s, query)
	assert.Nil(t, err)

	val, f := s.Get("cot-one")
	assert.True(t, f)
	assert.Equal(t, "123", val.(string))
}

func TestFormBodyProcessor_ActStoreSession1(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "four", Actions: []*Actor{{"store_into_session", []interface{}{"cot-four"}}},
		},
	}
	s := types.NewSession()
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(s, query)
	assert.Nil(t, err)

	val, f := s.Get("cot-four")
	assert.True(t, f)
	assert.Equal(t, "", val.(string))
}

func TestFormBodyProcessor_ActRandomString(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"rand_string", []interface{}{8}}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActTimestamp(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"current_timestamp", []interface{}{"ms"}}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestFormBodyProcessor_ActUUID(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"uuid", nil}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestRegisterGlobalActionFunc0(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	RegisterGlobalActionFunc("noname", func(s *types.Session, key string, data interface{}, params []interface{}) (interface{}, error) {
		d := data.(url.Values)
		d.Set(key, "ok!")
		return data, nil
	})
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"noname", nil}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestRegisterGlobalActionFunc1(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	RegisterGlobalActionFunc("noname", func(s *types.Session, key string, data interface{}, params []interface{}) (interface{}, error) {
		return data, errors.New("make error")
	})
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"noname", nil}},
		},
		{
			Key: "one", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	_, err := processor.Act(types.NewSession(), query)
	assert.Error(t, err)
}

func TestRegisterGlobalAssertFunc0(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	RegisterGlobalAssertFunc("noname", func(s *types.Session, key string, data interface{}, params []interface{}) error {
		return nil
	})
	var units = []*Unit{
		{
			Key: "one", Asserts: []*Assertor{{"noname", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	err := processor.Assert(types.NewSession(), query)
	assert.Nil(t, err)
}

func TestRegisterGlobalAssertFunc1(t *testing.T) {
	query := []byte("one=123&two=456&three=789")
	RegisterGlobalAssertFunc("noname", func(s *types.Session, key string, data interface{}, params []interface{}) error {
		return errors.New("make error")
	})
	var units = []*Unit{
		{
			Key: "one", Asserts: []*Assertor{{"noname", nil}},
		},
	}
	processor := NewFormBodyProcessor(units)
	err := processor.Assert(types.NewSession(), query)
	assert.Error(t, err)
}

func TestHTMLProcessor_Act(t *testing.T) {
	d := "window.variable.push('qrCodeId', '16030600036065765931')" +
		".push('browser', 'WEIXIN').push('browserName', '微信').push('memberId', 'E67R6dEFapt2w5nsHzTJD0suZkKtzMpF').push('merchantId', '03b1d2bf-3152-4da8-a10a-a93f6e00c90e')"
	var units = []*Unit{
		{
			Key: "one", Actions: []*Actor{{"regexp_picker", []interface{}{"qrCodeId', '(\\d+)'", int64(1), "cot-open-id"}}},
		},
	}
	s := types.NewSession()
	processor := NewHTMLProcessor(units)
	_, err := processor.Act(s, []byte(d))
	assert.Nil(t, err)

	val, f := s.Get("cot-open-id")
	assert.True(t, f)
	assert.Equal(t, val, "16030600036065765931")
}

func TestHTMLProcessor_Assert0(t *testing.T) {
	d := "hello i am havok"
	var units = []*Unit{
		{
			Key: "one", Asserts: []*Assertor{{"contain", []interface{}{"am", "havok"}}},
		},
	}
	processor := NewHTMLProcessor(units)
	err := processor.Assert(types.NewSession(), []byte(d))
	assert.Nil(t, err)
}

func TestHTMLProcessor_Assert1(t *testing.T) {
	d := "hello i am havok"
	var units = []*Unit{
		{
			Key: "one", Asserts: []*Assertor{{"contain", []interface{}{"am", "hehe"}}},
		},
	}
	processor := NewHTMLProcessor(units)
	err := processor.Assert(types.NewSession(), []byte(d))
	assert.Error(t, err)
}
