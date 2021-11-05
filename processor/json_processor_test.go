package processor

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/WoSai/havok/types"
	"github.com/BurntSushi/toml"
	"github.com/buger/jsonparser"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestActionSetBooleanValue(t *testing.T) {
	data := []byte(`{"name": "tony"}`)
	var err error
	var value interface{}
	var field []byte
	value = true

	field, err = jsoniter.Marshal(value)
	assert.Nil(t, err)

	data, err = jsonparser.Set(data, field, "age")
	assert.Nil(t, err)
	var dt jsonparser.ValueType
	field, dt, _, err = jsonparser.Get(data, "age")
	assert.Nil(t, err)
	assert.Equal(t, dt.String(), "boolean")
	b, _ := strconv.ParseBool(string(field))
	assert.Equal(t, value, b)
}

func TestActionSetStringValue(t *testing.T) {
	data := []byte(`{"name": "tony"}`)
	var err error
	var value interface{}
	var field []byte
	value = "asdasdasdasdasda"

	field, err = jsoniter.Marshal(value)
	assert.Nil(t, err)

	data, err = jsonparser.Set(data, field, "age")
	assert.Nil(t, err)
	var dt jsonparser.ValueType
	field, dt, _, err = jsonparser.Get(data, "age")
	assert.Nil(t, err)
	assert.Equal(t, dt.String(), "string")
	assert.Equal(t, value, string(field))
}

func TestActionSetNumberValue(t *testing.T) {
	data := []byte(`{"name": "tony"}`)
	var err error
	var value interface{}
	var field []byte
	value = 123.456

	field, err = jsoniter.Marshal(value)
	assert.Nil(t, err)

	data, err = jsonparser.Set(data, field, "age")
	assert.Nil(t, err)
	var dt jsonparser.ValueType
	field, dt, _, err = jsonparser.Get(data, "age")
	assert.Nil(t, err)
	assert.Equal(t, dt.String(), "number")

	f, err := strconv.ParseFloat(string(field), 10)
	assert.Equal(t, value, f)
}

func TestActionSetNullValue(t *testing.T) {
	data := []byte(`{"name": "tony"}`)
	var err error
	var value interface{}
	var field []byte
	value = nil

	field, err = jsoniter.Marshal(value)
	assert.Nil(t, err)

	data, err = jsonparser.Set(data, field, "age")
	assert.Nil(t, err)
	var dt jsonparser.ValueType
	field, dt, _, err = jsonparser.Get(data, "age")
	assert.Nil(t, err)
	assert.Equal(t, dt.String(), "null")
}

func TestActionParseJSONPath(t *testing.T) {
	key := "person.name.fullName"
	ks := parseJSONPath(key)
	if !reflect.DeepEqual(ks, []string{"person", "name", "fullName"}) {
		t.Log(ks)
		t.FailNow()
	}

	key = "person[0].name"
	ks = parseJSONPath(key)
	if !reflect.DeepEqual(ks, []string{"person", "[0]", "name"}) {
		t.Log(ks)
		t.FailNow()
	}

	key = "[0][1].name"
	ks = parseJSONPath(key)
	if !reflect.DeepEqual(ks, []string{"[0]", "[1]", "name"}) {
		t.Log(ks)
		t.FailNow()
	}

	key = "person[0][1].name"
	ks = parseJSONPath(key)
	if !reflect.DeepEqual(ks, []string{"person", "[0]", "[1]", "name"}) {
		t.Log(ks)
		t.FailNow()
	}
}

func TestActionConversion(t *testing.T) {
	//var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1}}`)
	var units = []*Unit{
		{Key: "result_code", Actions: []*Actor{{"set", []interface{}{"400"}}}},
	}
	var tomlContent = `
[[unit]]
key = "result_code"
	[[unit.actions]]
	action = "set"
	params = ["500"]

[[unit]]
key = "data.order_sn"
`

	data, err := yaml.Marshal(units)
	assert.Nil(t, err)
	fmt.Println(string(data))

	var u = &struct {
		Unit []*Unit
	}{}
	_, err = toml.Decode(tomlContent, u)
	assert.Nil(t, err)

	for _, un := range u.Unit {
		fmt.Println(un.Key, un.Actions)
	}
}

func TestJSONProcessor_ActSet(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1}}`)
	var units = []*Unit{
		{Key: "data.total_amount", Actions: []*Actor{
			{"set", []interface{}{200}}, {"print", nil}},
		},
		{
			Key: "*", Actions: []*Actor{{"print", nil}},
		},
	}
	var err error
	var val interface{}

	processor := NewJSONProcessor(units)
	val, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)

	f, _, _, err := jsonparser.Get(val.([]byte), "data", "total_amount")
	assert.Nil(t, err)
	var i float64
	err = jsoniter.Unmarshal(f, &i)
	assert.Nil(t, err)
	assert.Equal(t, 200, int(i))
}

func TestJSONProcessor_ActDelete(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"123"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Actions: []*Actor{{"delete", nil}},
		},
	}
	var err error
	var val interface{}
	processor := NewJSONProcessor(units)
	val, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)

	f, _, _, err := jsonparser.Get(val.([]byte), parseJSONPath(units[0].Key)...)
	assert.NotNil(t, err)
	assert.Nil(t, f)
}

func TestJSONProcessor_ActJsonContext(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"123"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Actions: []*Actor{{"json_context", []interface{}{"data.total_amount"}}},
		},
	}
	var err error
	var val interface{}
	processor := NewJSONProcessor(units)
	val, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)

	f, _, _, err := jsonparser.Get(val.([]byte), parseJSONPath(units[0].Key)...)
	assert.Nil(t, err)
	assert.Equal(t, "1", string(f))
}

func TestJSONProcessor_ActBuildString(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"123"}}`)
	var units = []*Unit{{
		Key: "data.reflect", Actions: []*Actor{
			{"print", nil},
			{"build_string", []interface{}{"hello_$0"}},
			{"print", nil},
		}}}

	var err error
	var val interface{}
	processor := NewJSONProcessor(units)
	val, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)

	f, _, _, err := jsonparser.Get(val.([]byte), parseJSONPath(units[0].Key)...)
	assert.Nil(t, err)
	assert.Equal(t, "hello_123", string(f))
}

func TestJSONProcessor_ActSetTimestampString(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"123"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Actions: []*Actor{{"current_timestamp", nil}},
		},
	}
	var err error
	var val interface{}
	processor := NewJSONProcessor(units)
	val, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)

	f, _, _, err := jsonparser.Get(val.([]byte), parseJSONPath(units[0].Key)...)
	assert.Nil(t, err)
	assert.Equal(t, 13, len(string(f)))
}

func TestJSONProcessor_ActSetTimestampInt(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"123"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Actions: []*Actor{{"current_timestamp", []interface{}{"ms", "int"}}},
		},
	}
	var err error
	var val interface{}
	processor := NewJSONProcessor(units)
	val, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)

	f, err := jsonparser.GetInt(val.([]byte), parseJSONPath(units[0].Key)...)
	assert.Nil(t, err)
	assert.Equal(t, reflect.TypeOf(f).String(), "int64")
	assert.Equal(t, 13, len(fmt.Sprint(f)))
}

func TestJSONProcessor_ActSetUUID(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"123"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Actions: []*Actor{{"uuid", nil}},
		},
	}
	var err error
	var val interface{}
	processor := NewJSONProcessor(units)
	val, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)

	f, _, _, err := jsonparser.Get(val.([]byte), parseJSONPath(units[0].Key)...)
	assert.Nil(t, err)
	assert.Equal(t, 36, len(string(f)))
}

func TestJSONProcessor_ActSetReverse(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"123"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Actions: []*Actor{{"reverse", nil}},
		},
	}
	var err error
	var val interface{}
	processor := NewJSONProcessor(units)
	val, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)

	f, _, _, err := jsonparser.Get(val.([]byte), parseJSONPath(units[0].Key)...)
	assert.Nil(t, err)
	assert.Equal(t, "321", string(f))
}

func TestJSONProcessor_ActStoreSession(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"haha"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Actions: []*Actor{{"store_into_session", []interface{}{"cot-reflect"}}},
		},
	}
	var err error
	s := types.NewSession()
	processor := NewJSONProcessor(units)
	_, err = processor.Act(s, body)
	assert.Nil(t, err)

	v, f := s.Get("cot-reflect")
	assert.True(t, f)
	assert.Equal(t, "haha", string(v.([]byte)))
}

func TestJSONProcessor_ActSetSliceToString(t *testing.T) {
	var body = []byte(`{"method": "getAccountRecordReportProxy","params": [{"reportLoadTypes": ["TOTAL_SUMMARY,TIME_DAY_SUMMARY"]}]}`)
	var units = []*Unit{
		{
			Key: "params[0].reportLoadTypes", Actions: []*Actor{{"slice_string", nil}},
		},
	}
	var err error
	var val interface{}
	processor := NewJSONProcessor(units)
	val, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)

	f, _, _, err := jsonparser.Get(val.([]byte), parseJSONPath(units[0].Key)...)
	assert.Nil(t, err)
	assert.Equal(t, "TOTAL_SUMMARY,TIME_DAY_SUMMARY", string(f))
}

func TestJSONProcessor_ActGetFromSession(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"haha"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Actions: []*Actor{{"get_from_session", []interface{}{"cot-reflect"}}},
		},
	}
	var err error
	var val interface{}
	s := types.NewSession()
	s.Put("cot-reflect", "wtf")
	processor := NewJSONProcessor(units)
	val, err = processor.Act(s, body)
	assert.Nil(t, err)

	f, _, _, err := jsonparser.Get(val.([]byte), parseJSONPath(units[0].Key)...)
	assert.Nil(t, err)
	assert.Equal(t, "wtf", string(f))
}

func TestJSONProcessor_ActPrintAll(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"haha"}}`)
	var units = []*Unit{
		{
			Key: "*", Actions: []*Actor{{"print", nil}},
		},
	}
	var err error
	processor := NewJSONProcessor(units)
	_, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_ActPrintKey(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"haha"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Actions: []*Actor{{"print", nil}},
		},
	}
	var err error
	processor := NewJSONProcessor(units)
	_, err = processor.Act(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestRegisterActionFuncInJSONProcessor(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"haha"}}`)
	RegisterActionFuncInJSONProcessor("noname", func(s *types.Session, key string, data interface{}, params []interface{}) (interface{}, error) {
		return []byte("123"), nil
	})
	units := []*Unit{
		{Key: "data.reflect", Actions: []*Actor{{"noname", nil}}},
	}
	processor := NewJSONProcessor(units)
	val, err := processor.Act(types.NewSession(), body)
	assert.Nil(t, err)

	assert.Equal(t, "123", string(val.([]byte)))
}

func TestJSONProcessor_AssertEqualString(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"haha"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"eq", []interface{}{"haha"}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertEqualInt(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456789}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"eq", []interface{}{int64(123456789)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertEqualFloat64(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":12.75}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"eq", []interface{}{float64(12.75)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertNotEqualString(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"haha"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"ne", []interface{}{"haha"}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertNotEqualInt(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456789}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"ne", []interface{}{int64(123456789)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertNotEqualFloat64(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":12.75}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"ne", []interface{}{float64(12.75)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertIntGreater0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456789}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"gt", []interface{}{int64(123456788)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertIntGreater1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456789}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"gt", []interface{}{int64(123456789)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertFloatGreater0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"gt", []interface{}{float64(37.24)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertFloatGreater1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"gt", []interface{}{float64(37.25)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertIntGreaterAndEqual0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456789}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"ge", []interface{}{int64(123456788)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertIntGreaterAndEqual1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456789}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"ge", []interface{}{int64(123456789)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertIntGreaterAndEqual2(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456788}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"ge", []interface{}{int64(123456789)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertFloatGreaterAndEqual0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"ge", []interface{}{float64(37.24)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertFloatGreaterAndEqual1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"ge", []interface{}{float64(37.25)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertFloatGreaterAndEqual2(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.24}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"ge", []interface{}{float64(37.25)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertIntLess0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456788}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"lt", []interface{}{int64(123456789)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertIntLess1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456788}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"lt", []interface{}{int64(123456788)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertIntLess2(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456788}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"lt", []interface{}{int64(123456787)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertFloatLess0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"lt", []interface{}{float64(37.26)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertFloatLess1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"lt", []interface{}{float64(37.25)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertFloatLess2(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"lt", []interface{}{float64(37.24)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertIntLessAndEqual0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456788}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"le", []interface{}{int64(123456789)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertIntLessAndEqual1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456788}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"le", []interface{}{int64(123456788)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertIntLessAndEqual2(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123456788}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"le", []interface{}{int64(123456787)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertFloatLessAndEqual0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"le", []interface{}{float64(37.26)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertFloatLessAndEqual1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"le", []interface{}{float64(37.25)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertFloatLessAndEqual2(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"le", []interface{}{float64(37.24)}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertExist0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"exist", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertExist1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect1", Asserts: []*Assertor{{"exist", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertNotExist0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect1", Asserts: []*Assertor{{"not_exist", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertNotExist1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":37.25}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"not_exist", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertNull0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":null}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"null", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertNull1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"null", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertNotNull0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":123}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"not_null", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertNotNull1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":null}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"not_null", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertTrue0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":true}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"true", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertTrue1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":false}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"true", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertTrue2(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":1234}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"true", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertFalse0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":false}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"false", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertFalse1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":true}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"false", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertFalse2(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":1234}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"false", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertEmpty0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":""}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"empty", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertEmpty1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"123"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"empty", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertNotEmpty0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"123"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"not_empty", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertNotEmpty1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":""}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"not_empty", nil}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertIn0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"1234"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"in", []interface{}{"1234"}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertIn1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"1234"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"in", []interface{}{"5678", "1234"}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertIn2(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"1234"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"in", []interface{}{"5678"}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertIn3(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"1234"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"in", []interface{}{"5678", "0123"}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertNotIn0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"1234"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"not_in", []interface{}{"1234"}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertNotIn1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"1234"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"not_in", []interface{}{"5678", "1234"}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}

func TestJSONProcessor_AssertNotIn2(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"1234"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"not_in", []interface{}{"5678"}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestJSONProcessor_AssertNotIn3(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"1234"}}`)
	var units = []*Unit{
		{
			Key: "data.reflect", Asserts: []*Assertor{{"not_in", []interface{}{"5678", "01234"}}},
		},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestRegisterAssertFuncInJSONProcessor0(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"haha"}}`)
	RegisterAssertFuncInJSONProcessor("noname", func(s *types.Session, key string, data interface{}, params []interface{}) error {
		return nil
	})
	units := []*Unit{
		{Key: "data.reflect", Asserts: []*Assertor{{"noname", nil}}},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Nil(t, err)
}

func TestRegisterAssertFuncInJSONProcessor1(t *testing.T) {
	var body = []byte(`{"result_code": 200, "data": {"order_sn": "1000000000", "total_amount": 1, "reflect":"haha"}}`)
	RegisterAssertFuncInJSONProcessor("noname", func(s *types.Session, key string, data interface{}, params []interface{}) error {
		return errors.New("make error")
	})
	units := []*Unit{
		{Key: "data.reflect", Asserts: []*Assertor{{"noname", nil}}},
	}
	processor := NewJSONProcessor(units)
	err := processor.Assert(types.NewSession(), body)
	assert.Error(t, err)
}
