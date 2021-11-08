package processor

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"regexp"
	"strconv"
	"strings"

	"github.com/wosai/havok/types"
	"github.com/buger/jsonparser"
	jsoniter "github.com/json-iterator/go"
	"go.uber.org/zap"
)

type (
	JSONProcessor struct {
		keys  map[string][]string
		units Units
	}
)

var (
	jsonPathCompiler         = regexp.MustCompile(`\[[\d]]+`)
	extendJSONProcessorFuncs = &struct {
		actions map[ActionType]ActionFunc
		asserts map[AssertType]AssertFunc
	}{actions: make(map[ActionType]ActionFunc), asserts: make(map[AssertType]AssertFunc)}
)

// NewJSONProcessor JSONProcessor的构造函数，需要传入Units，可传nil
func NewJSONProcessor(us Units) *JSONProcessor {
	return &JSONProcessor{keys: make(map[string][]string), units: us}
}

// WithUnits 更新JSONProcessor的Units
func (jp *JSONProcessor) WithUnits(us Units) {
	jp.units = us
}

// Act JSONProcessor对Action的处理实现
func (jp *JSONProcessor) Act(s *types.Session, body interface{}) (interface{}, error) {
	data, ok := body.([]byte)
	if !ok {
		return nil, errors.New(fmt.Sprintf("unexpected data type: %T", body))
	}

	var err error
	var field []byte

	for _, unit := range jp.units {
		for _, action := range unit.Actions {
			switch action.Method {
			case actionSliceToString:
				field, _, _, err = jsonparser.Get(data, jp.getJSONPath(unit.Key)...)
				if err != nil {
					return nil, err
				}
				if !strings.Contains(string(field), "[") {
					continue
				}
				var tmp []string
				err := jsoniter.Unmarshal(field, &tmp)
				if err != nil {
					return nil, err
				}
				field, err = jsoniter.Marshal(strings.Join(tmp, ","))
				if err != nil {
					return nil, err
				}
				data, err = jsonparser.Set(data, field, jp.getJSONPath(unit.Key)...)

			case actionSetValue:
				if len(action.Params) != 1 {
					return nil, ErrMissedRequiredParams
				}
				field, err = jsoniter.Marshal(action.Params[0])
				if err != nil {
					return nil, ErrIncorrectParamType
				}
				data, err = jsonparser.Set(data, field, jp.getJSONPath(unit.Key)...)
				if err != nil {
					Logger.Info("actionSetValue occurs error", zap.Error(err))
					return nil, err
				}

			case actionDelete:
				data = jsonparser.Delete(data, jp.getJSONPath(unit.Key)...)

			case "json_context":
				var other string
				other, err = checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				field, _, _, err = jsonparser.Get(data, jp.getJSONPath(other)...)
				if err != nil {
					return nil, err
				}
				data, err = jsonparser.Set(data, field, jp.getJSONPath(unit.Key)...)
				if err != nil {
					return nil, err
				}

			case actionSetCurrentTimestamp:
				pre := "ms"
				as := "string"
				var ok bool
				if len(action.Params) >= 1 {
					pre, ok = action.Params[0].(string)
					if !ok {
						return nil, ErrIncorrectParamType
					}
				}
				if len(action.Params) == 2 {
					as, ok = action.Params[1].(string)
					if !ok {
						return nil, ErrIncorrectParamType
					}
				}
				ts := currentTimestamp(pre)
				if as == "int" {
					field, err = jsoniter.Marshal(ts)
					if err != nil {
						return nil, err
					}
				} else {
					field, err = jsoniter.Marshal(strconv.FormatInt(ts, 10))
					if err != nil {
						return nil, err
					}
				}
				data, err = jsonparser.Set(data, field, jp.getJSONPath(unit.Key)...)
				if err != nil {
					return nil, err
				}

			case actionRandString:
				length := 8
				if len(action.Params) > 0 {
					v, err := checkParserToInt(action.Params[0])
					if err != nil {
						return nil, err
					} else {
						length = v
					}
				}
				field, err = jsoniter.Marshal(RandStringBytesMaskImprSrc(length))
				if err != nil {
					return nil, err
				}
				data, err = jsonparser.Set(data, field, jp.getJSONPath(unit.Key)...)

			case actionSetUUID:
				field, err = jsoniter.Marshal(uuid.NewString())
				if err != nil {
					return nil, err
				}
				data, err = jsonparser.Set(data, field, jp.getJSONPath(unit.Key)...)
				if err != nil {
					return nil, err
				}

			case actionReverse:
				field, _, _, err = jsonparser.Get(data, jp.getJSONPath(unit.Key)...)
				if err != nil {
					return nil, err
				}
				temp := reverse(string(field))
				field, err = jsoniter.Marshal(temp)
				if err != nil {
					return nil, err
				}
				data, err = jsonparser.Set(data, field, jp.getJSONPath(unit.Key)...)
				if err != nil {
					return nil, err
				}

			case actionStoreIntoSession:
				var key string
				key, err = checkIfStringType(action.Params)
				field, _, _, err = jsonparser.Get(data, jp.getJSONPath(unit.Key)...)
				if err != nil {
					Logger.Info("actionStoreIntoSession occurs error", zap.Error(err), zap.String("key", unit.Key))
					return nil, err
				}
				s.Put(key, field)

			case actionGetFromSession:
				var key string
				var b []byte
				key, err = checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				val, ok := s.Get(key)
				if !ok {
					return nil, errors.New("found no item from session with key: " + key)
				}
				// jsonProcessor的val都作为[]byte
				if b, ok = val.([]byte); !ok {
					if b, err = jsoniter.Marshal(val); err != nil {
						return nil, err
					}
				}
				data, err = jsonparser.Set(data, b, jp.getJSONPath(unit.Key)...)
				if err != nil {
					return nil, err
				}

			case actionPrint:
				if unit.Key == "*" {
					Logger.Info("print data", zap.ByteString("data", data))
				} else {
					field, _, _, err = jsonparser.Get(data, jp.getJSONPath(unit.Key)...)
					Logger.Info("print value as string", zap.ByteString(unit.Key, field))
				}

			case actionBuildString:
				var dt jsonparser.ValueType
				field, dt, _, err = jsonparser.Get(data, jp.getJSONPath(unit.Key)...)
				if err != nil {
					return nil, err
				}
				if dt != jsonparser.String {
					return nil, ErrUnexpectedDataType
				}
				var expr string
				expr, err = checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				field, err = jsoniter.Marshal(strings.Replace(expr, buildStringPlaceHolder, string(field), -1))
				if err != nil {
					return nil, err
				}
				data, err = jsonparser.Set(data, field, jp.getJSONPath(unit.Key)...)
				if err != nil {
					return nil, err
				}

			default:
				if f, err := getProperActionFunc(extendJSONProcessorFuncs.actions, action.Method); err == nil {
					if ret, err := f(s, unit.Key, data, action.Params); err == nil {
						if data, ok = ret.([]byte); !ok {
							return nil, ErrUnexpectedDataType
						}
					} else {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
		}
	}
	return data, err
}

// Assert JSONProcessor的断言函数
func (jp *JSONProcessor) Assert(s *types.Session, body interface{}) error {
	data, ok := body.([]byte)
	if !ok {
		return errors.New(fmt.Sprintf("unexpected data type: %T", body))
	}
	var err error

	for _, unit := range jp.units {
		var val []byte
		var vt jsonparser.ValueType
		val, vt, _, err = jsonparser.Get(data, jp.getJSONPath(unit.Key)...) // 所有断言行为中，仅有一次对key取值

		for _, assert := range unit.Asserts {
			switch assert.Method {
			case assertEqual:
				if err = doAssertEqual(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertNotEqual:
				if err = doAssertNotEqual(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertGreater:
				if err = doAssertNumberValues(unit.Key, val, vt, err, "gt", assert.Params); err != nil {
					return err
				}

			case assertGreaterOrEqual:
				if err = doAssertNumberValues(unit.Key, val, vt, err, "ge", assert.Params); err != nil {
					return err
				}

			case assertLessOrEqual:
				if err = doAssertNumberValues(unit.Key, val, vt, err, "le", assert.Params); err != nil {
					return err
				}

			case assertLess:
				if err = doAssertNumberValues(unit.Key, val, vt, err, "lt", assert.Params); err != nil {
					return err
				}

			case assertExist:
				if err = doAssertExist(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertNotExist:
				if err = doAssertNotExist(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertNull:
				if err = doAssertNull(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertNotNull:
				if err = doAssertNotNull(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertTrue:
				if err = doAssertTrue(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertFalse:
				if err = doAssertFalse(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertEmpty:
				if err = doAssertEmpty(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertNotEmpty:
				if err = doAssertNotEmpty(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertIn:
				if err = doAssertIn(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			case assertNotIn:
				if err = doAssertNotIn(unit.Key, val, vt, err, assert.Params); err != nil {
					return err
				}

			default:
				if f, err := getProperAssertFunc(extendJSONProcessorFuncs.asserts, assert.Method); err == nil {
					if err = f(s, unit.Key, data, assert.Params); err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}
	}
	return err
}

// getJSONPath 将key解析成buger/jsonparse支持的path
func (jp *JSONProcessor) getJSONPath(key string) []string {
	if val, ok := jp.keys[key]; ok {
		return val
	}
	v := parseJSONPath(key)
	jp.keys[key] = v
	return v
}

func parseJSONPath(key string) []string {
	var ret []string
	ks := strings.Split(key, ".")

	for _, s := range ks {
		ret = append(ret, splitStringByIndexs(s, jsonPathCompiler.FindAllStringIndex(s, -1)...)...)
	}
	return ret
}

func splitStringByIndexs(s string, indexs ...[]int) []string {
	if indexs == nil || len(indexs) == 0 {
		return []string{s}
	}
	length := len(s)
	var ret []string
	var offset int

	for _, index := range indexs {
		start := index[0]
		end := index[1]

		if offset < start { // 分割位置落后于偏移量
			ret = append(ret, s[offset:start])
			offset = start
		}

		if offset == start { // 分割位置与目前一致
			ret = append(ret, s[start:end])
			offset = end
		}
	}

	if offset < length {
		ret = append(ret, s[offset:length])
	}

	return ret
}

func doAssertEqual(key string, value []byte, dt jsonparser.ValueType, err error, params []interface{}) error {
	if err != nil {
		return err
	}
	if len(params) == 0 {
		return ErrMissedRequiredParams
	}
	expect := params[0]
	switch dt {
	case jsonparser.String:
		actual := string(value)
		expected := expect.(string)
		if actual == expected {
			return nil
		}
		return NewAssertionError(key, assertEqual, actual, expected)
	case jsonparser.Number:
		v, t, err := tryConvertNumber(value)
		if err != nil {
			return err
		}
		switch t {
		case "int64":
			actual := v.(int64)
			switch a := expect.(type) {
			case int64:
				if actual == a {
					return nil
				} else {
					return NewAssertionError(key, assertEqual, actual, a)
				}
			case float64:
				if actual == int64(a) {
					return nil
				} else {
					return NewAssertionError(key, assertEqual, actual, int64(a))
				}
			default:
				return NewAssertionError(key, assertEqual, actual, a)
			}
		case "float64":
			actual := v.(float64)
			switch a := expect.(type) {
			case float64:
				if actual == a {
					return nil
				} else {
					return NewAssertionError(key, assertEqual, actual, a)
				}
			case int64:
				if actual == float64(a) {
					return nil
				} else {
					return NewAssertionError(key, assertEqual, actual, float64(a))
				}
			default:
				return NewAssertionError(key, assertEqual, actual, a)
			}
		default:
			return ErrIncorrectParamType
		}
	default:
		return ErrUnexpectedDataType
	}
}

func doAssertNotEqual(key string, value []byte, dt jsonparser.ValueType, err error, params []interface{}) error {
	if err != nil {
		return err
	}
	if len(params) == 0 {
		return ErrMissedRequiredParams
	}
	expect := params[0]
	switch dt {
	case jsonparser.String:
		a := string(value)
		e := expect.(string)
		if a != e {
			return nil
		}
		return NewAssertionError(key, assertNotEqual, a, e)
	case jsonparser.Number:
		v, t, err := tryConvertNumber(value)
		if err != nil {
			return err
		}
		switch t {
		case "int64":
			actual := v.(int64)
			switch a := expect.(type) {
			case int64:
				if actual != a {
					return nil
				} else {
					return NewAssertionError(key, assertNotEqual, actual, a)
				}
			case float64:
				if actual != int64(a) {
					return nil
				} else {
					return NewAssertionError(key, assertNotEqual, actual, int64(a))
				}
			default:
				return NewAssertionError(key, assertNotEqual, actual, a)
			}
		case "float64":
			actual := v.(float64)
			switch a := expect.(type) {
			case float64:
				if actual != a {
					return nil
				} else {
					return NewAssertionError(key, assertNotEqual, actual, a)
				}
			case int64:
				if actual != float64(a) {
					return nil
				} else {
					return NewAssertionError(key, assertNotEqual, actual, float64(a))
				}
			default:
				return NewAssertionError(key, assertNotEqual, actual, a)
			}
		default:
			return ErrIncorrectParamType
		}
	default:
		return ErrUnexpectedDataType
	}
}

func doAssertNumberValues(key string, val []byte, dt jsonparser.ValueType, err error, opt string, params []interface{}) error {
	if len(params) == 0 {
		return ErrMissedRequiredParams
	}
	if err != nil {
		return err
	}
	if dt != jsonparser.Number {
		return ErrUnexpectedDataType
	}
	f, t, err := tryConvertNumber(val)
	if err != nil {
		return err
	}

	switch t {
	case "int64":
		expected := params[0].(int64)
		actual := f.(int64)
		switch opt {
		case "ge":
			if actual >= expected {
				return nil
			}
			return NewAssertionError(key, assertGreaterOrEqual, actual, expected)
		case "gt":
			if actual > expected {
				return nil
			}
			return NewAssertionError(key, assertGreater, actual, expected)
		case "lt":
			if actual < expected {
				return nil
			}
			return NewAssertionError(key, assertLess, actual, expected)
		case "le":
			if actual <= expected {
				return nil
			}
			return NewAssertionError(key, assertLessOrEqual, actual, expected)
		default:
			return ErrUnsupportedExpression
		}
	case "float64":
		expected := params[0].(float64)
		actual := f.(float64)
		switch opt {
		case "ge":
			if actual >= expected {
				return nil
			}
			return NewAssertionError(key, assertGreaterOrEqual, actual, expected)
		case "gt":
			if actual > expected {
				return nil
			}
			return NewAssertionError(key, assertGreater, actual, expected)
		case "lt":
			if actual < expected {
				return nil
			}
			return NewAssertionError(key, assertLess, actual, expected)
		case "le":
			if actual <= expected {
				return nil
			}
			return NewAssertionError(key, assertLessOrEqual, actual, expected)
		default:
			return ErrUnsupportedExpression
		}
	default:
		return ErrIncorrectParamType
	}
}

func doAssertExist(_ string, _ []byte, dt jsonparser.ValueType, err error, _ []interface{}) error {
	if dt != jsonparser.NotExist {
		return nil
	} // 如果字段不存在，肯定err != nil
	return err
}

func doAssertNotExist(key string, v []byte, dt jsonparser.ValueType, _ error, _ []interface{}) error {
	if dt == jsonparser.NotExist {
		return nil
	}
	return NewAssertionError(key, assertNotExist, string(v), nil)
}

func doAssertTrue(key string, v []byte, dt jsonparser.ValueType, err error, _ []interface{}) error {
	switch {
	case err != nil:
		return err
	case dt == jsonparser.Boolean:
		b, err := strconv.ParseBool(string(v))
		if err != nil {
			return err
		}
		if b {
			return nil
		}
		return NewAssertionError(key, assertTrue, b, nil)
	default:
		return ErrUnexpectedDataType
	}
}

func doAssertFalse(key string, v []byte, dt jsonparser.ValueType, err error, _ []interface{}) error {
	switch {
	case err != nil:
		return err
	case dt == jsonparser.Boolean:
		b, err := strconv.ParseBool(string(v))
		if err != nil {
			return err
		}
		if !b {
			return nil
		}
		return NewAssertionError(key, assertFalse, b, nil)
	default:
		return ErrUnexpectedDataType
	}
}

func doAssertNull(key string, v []byte, dt jsonparser.ValueType, err error, _ []interface{}) error {
	if dt == jsonparser.Null {
		return nil
	}
	if err != nil {
		return err
	}
	return NewAssertionError(key, assertNull, string(v), nil)
}

func doAssertNotNull(key string, v []byte, dt jsonparser.ValueType, err error, _ []interface{}) error {
	if err != nil {
		return err
	}
	if dt != jsonparser.Null {
		return nil
	}
	return NewAssertionError(key, assertNotNull, string(v), nil)
}

func doAssertEmpty(key string, v []byte, dt jsonparser.ValueType, err error, _ []interface{}) error {
	if err != nil {
		return err
	}
	if dt != jsonparser.String {
		return ErrUnexpectedDataType
	}
	if string(v) == "" {
		return nil
	}
	return NewAssertionError(key, assertEmpty, string(v), "")
}

func doAssertNotEmpty(key string, v []byte, dt jsonparser.ValueType, err error, _ []interface{}) error {
	if err != nil {
		return err
	}
	if dt != jsonparser.String {
		return ErrUnexpectedDataType
	}
	if string(v) != "" {
		return nil
	}
	return NewAssertionError(key, assertNotEmpty, string(v), nil)
}

func doAssertIn(key string, v []byte, dt jsonparser.ValueType, err error, params []interface{}) error {
	if err != nil {
		return err
	}
	var temp []string
	if dt == jsonparser.String {
		actual := string(v)
		for _, param := range params {
			if p, ok := param.(string); ok {
				temp = append(temp, p)
				if p == actual {
					return nil
				}
			} else {
				return ErrIncorrectParamType
			}
		}
		return NewAssertionError(key, assertIn, actual, temp)
	}
	return ErrUnexpectedDataType
}

func doAssertNotIn(key string, v []byte, dt jsonparser.ValueType, err error, params []interface{}) error {
	if err != nil {
		return err
	}
	var temp []string
	if dt == jsonparser.String {
		actual := string(v)
		for _, param := range params {
			if p, ok := param.(string); ok {
				temp = append(temp, p)
				if p == actual {
					return NewAssertionError(key, assertNotIn, actual, temp)
				}
			}
		}
		return nil
	}
	return ErrUnexpectedDataType
}

func tryConvertNumber(v []byte) (interface{}, string, error) {
	s := string(v)
	i, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return i, "int64", nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return f, "float64", nil
	}
	return nil, "unknown", ErrFailedConvertToStringOrNumber
}

// RegisterActionFuncInJSONProcessor 注册第三方实现的ActionFunc
func RegisterActionFuncInJSONProcessor(n ActionType, f ActionFunc) {
	extendJSONProcessorFuncs.actions[n] = f
}

// RegisterAssertFuncInJSONProcessor 注册第三方实现AssertFunc
func RegisterAssertFuncInJSONProcessor(n AssertType, f AssertFunc) {
	extendJSONProcessorFuncs.asserts[n] = f
}
