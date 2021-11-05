package processor

import (
	"fmt"
	"github.com/google/uuid"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"reflect"
	"strings"

	"errors"

	"github.com/WoSai/havok/types"
	"go.uber.org/zap"
)

type (
	// Processor 对http响应报文的处理器
	Processor interface {
		// WithUnits 加载Units配置
		WithUnits(Units)
		// Act Action执行函数
		Act(s *types.Session, data interface{}) (interface{}, error)
		// Assert 断言函数
		Assert(s *types.Session, data interface{}) error
	}

	valuesProcessor struct {
		units Units
	}

	// URLQueryProcessor url query string处理器
	URLQueryProcessor struct {
		*valuesProcessor
	}

	// FormBodyProcessor form-post 报文处理器
	FormBodyProcessor struct {
		*valuesProcessor
	}

	// HTMLProcessor 通用的html报文处理器
	HTMLProcessor struct {
		units     Units
		compilers map[string]*regexp.Regexp
	}
)

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits

	actionSliceToString       = "slice_string"
	actionRandString          = "rand_string"
	actionPrint               = "print"
	actionSetValue            = "set"
	actionDelete              = "delete"
	actionGetFromSession      = "get_from_session"
	actionStoreIntoSession    = "store_into_session"
	actionSetCurrentTimestamp = "current_timestamp"
	actionSetUUID             = "uuid"
	actionReverse             = "reverse"
	actionBuildString         = "build_string"

	assertEqual          = "eq" // 适用于string或者number
	assertNotEqual       = "ne" // 适用于string或者number
	assertGreater        = "gt" // 仅适用于number
	assertLess           = "lt" // 仅适用于number
	assertGreaterOrEqual = "ge" // 仅适用于number
	assertLessOrEqual    = "le" // 仅适用于number
	assertExist          = "exist"
	assertNotExist       = "not_exist"
	assertTrue           = "true"
	assertFalse          = "false"
	assertNull           = "null"
	assertNotNull        = "not_null"
	assertEmpty          = "empty"     // string
	assertNotEmpty       = "not_empty" // string
	assertIn             = "in"        // string
	assertNotIn          = "not_in"    // string

	buildStringPlaceHolder = `$0`
)

var (
	extendURLQueryProcessorFuncs = &struct {
		actions map[ActionType]ActionFunc
		asserts map[AssertType]AssertFunc
	}{
		actions: make(map[ActionType]ActionFunc),
		asserts: make(map[AssertType]AssertFunc),
	}

	extendFormBodyProcessorFuncs = &struct {
		actions map[ActionType]ActionFunc
		asserts map[AssertType]AssertFunc
	}{
		actions: make(map[ActionType]ActionFunc),
		asserts: make(map[AssertType]AssertFunc),
	}

	extendHTMLProcessorFuncs = &struct {
		actions map[ActionType]ActionFunc
		asserts map[AssertType]AssertFunc
	}{
		actions: make(map[ActionType]ActionFunc),
		asserts: make(map[AssertType]AssertFunc),
	}

	extendGlobalProcessorFunc = &struct {
		actions map[ActionType]ActionFunc
		asserts map[AssertType]AssertFunc
	}{
		actions: make(map[ActionType]ActionFunc),
		asserts: make(map[AssertType]AssertFunc),
	}
)

// RandStringBytesMaskImprSrc 生成指定长度的随机字符串
func RandStringBytesMaskImprSrc(n int) []byte {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return b
}

func currentTimestamp(precision string) int64 {
	now := time.Now().UnixNano()
	switch precision {
	case "mcs":
		return now / 1e3
	case "ms":
		return now / 1e6
	case "s":
		return now / 1e9
	default:
		return now
	}
}

func newValuesProcessor(us Units) *valuesProcessor {
	return &valuesProcessor{units: us}
}

func (vp *valuesProcessor) WithUnits(us Units) {
	vp.units = us
}

func (vp *valuesProcessor) act(s *types.Session, val interface{}, child string) (interface{}, error) {
	values, ok := val.(url.Values)
	if !ok {
		return nil, errors.New(fmt.Sprintf("unexpexted data type: %T", val))
	}

	for _, unit := range vp.units {
		for _, action := range unit.Actions {
			switch action.Method {
			case actionPrint:
				Logger.Info("print key", zap.Strings(unit.Key, values[unit.Key]))

			case actionSetValue:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				values.Set(unit.Key, v)

			case actionReverse:
				temp := reverse(values.Get(unit.Key))
				values.Set(unit.Key, temp)

			case "add":
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				values.Add(unit.Key, v)

			case actionDelete:
				values.Del(unit.Key)

			case actionGetFromSession:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				if val, loaded := s.Get(v); loaded {
					if s, ok := val.(string); ok {
						values.Set(unit.Key, s)
					} else {
						return nil, ErrIncorrectParamType
					}
				} else {
					return nil, ErrBadParamValue
				}

			case actionStoreIntoSession:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				s.Put(v, values.Get(unit.Key))

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
				values.Set(unit.Key, string(RandStringBytesMaskImprSrc(length)))

			case actionSetCurrentTimestamp:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				values.Set(unit.Key, strconv.FormatInt(currentTimestamp(v), 10))

			case actionSetUUID:
				values.Set(unit.Key, uuid.NewString())

			default:
				var f ActionFunc
				var err error
				switch child {
				case "query":
					f, err = getProperActionFunc(extendURLQueryProcessorFuncs.actions, action.Method)
				case "form":
					f, err = getProperActionFunc(extendFormBodyProcessorFuncs.actions, action.Method)
				}
				if err != nil {
					return nil, err
				}
				if _, err = f(s, unit.Key, values, action.Params); err != nil {
					return nil, err
				}
			}
		}
	}
	return values, nil
}

func (vp *valuesProcessor) assert(s *types.Session, v interface{}, child string) error {
	for _, unit := range vp.units {
		for _, assert := range unit.Asserts {
			switch assert.Method {
			default:
				var f AssertFunc
				var err error

				switch child {
				case "query":
					f, err = getProperAssertFunc(extendURLQueryProcessorFuncs.asserts, assert.Method)
				case "form":
					f, err = getProperAssertFunc(extendFormBodyProcessorFuncs.asserts, assert.Method)
				}
				if err != nil {
					return err
				}
				if err = f(s, unit.Key, v, assert.Params); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// NewURLQueryProcessor URLQueryProcessor构造函数
func NewURLQueryProcessor(us Units) *URLQueryProcessor {
	return &URLQueryProcessor{newValuesProcessor(us)}
}

func (qp *URLQueryProcessor) Act(s *types.Session, v interface{}) (interface{}, error) {
	return qp.valuesProcessor.act(s, v, "query")
}

func (qp *URLQueryProcessor) Assert(s *types.Session, v interface{}) error {
	return qp.valuesProcessor.assert(s, v, "query")
}

// NewFormBodyProcessor FormBodyProcessor构造函数
func NewFormBodyProcessor(us Units) *FormBodyProcessor {
	return &FormBodyProcessor{newValuesProcessor(us)}
}

// Act FormBodyProcessor对Processor.Act的实现
func (fp *FormBodyProcessor) Act(s *types.Session, v interface{}) (interface{}, error) {
	vv, ok := v.([]byte)
	if !ok {
		return nil, ErrUnexpectedDataType
	}
	val, err := url.ParseQuery(string(vv))
	if err != nil {
		return nil, ErrBadParamValue
	}
	return fp.valuesProcessor.act(s, val, "form")
}

// Assert FormBodyProcessor对Process.Assert的实现
func (fp *FormBodyProcessor) Assert(s *types.Session, v interface{}) error {
	vv, ok := v.([]byte)
	if !ok {
		return ErrUnexpectedDataType
	}
	val, err := url.ParseQuery(string(vv))
	if err != nil {
		return ErrBadParamValue
	}
	return fp.valuesProcessor.assert(s, val, "form")
}

// NewHTMLProcessor HTMLProcessor的构造函数
func NewHTMLProcessor(us Units) *HTMLProcessor {
	return &HTMLProcessor{units: us, compilers: make(map[string]*regexp.Regexp)}
}

func (ht *HTMLProcessor) WithUnits(us Units) {
	ht.units = us
}

func (ht *HTMLProcessor) Act(s *types.Session, body interface{}) (interface{}, error) {
	data, ok := body.([]byte)
	if !ok {
		return nil, ErrIncorrectParamType
	}

	for _, unit := range ht.units {
		for _, action := range unit.Actions {
			switch action.Method {
			case "regexp_picker":
				var index int64
				if len(action.Params) != 3 {
					return nil, ErrMissedRequiredParams
				}
				expr, ok := action.Params[0].(string)
				if !ok {
					return nil, ErrIncorrectParamType
				}

				switch t := action.Params[1].(type) {
				case int64:
					index = t
				case float64:
					index = int64(t)
				default:
					return nil, errors.New(fmt.Sprintf("unknown type: %s", reflect.TypeOf(t)))
				}

				k, ok := action.Params[2].(string)
				if !ok {
					return nil, ErrIncorrectParamType
				}
				re, exist := ht.compilers[expr]
				if !exist {
					c, err := regexp.Compile(expr)
					if err != nil {
						return nil, err
					}
					ht.compilers[expr] = c
					re = c
				}
				matches := re.FindStringSubmatch(string(data))
				if len(matches) >= 2 {
					matched := string(matches[index])
					if matched == "" {
						return nil, errors.New(fmt.Sprintf("no matched string, expr: %s", re.String()))
					}
					s.Put(k, matched)
				} else {
					fmt.Println("error formate matched list", zap.Strings("target", matches))
					return nil, errors.New(fmt.Sprintf("error formate matched string, expr: %s, org: %s", re.String(), string(data)))
				}
			default:
				if f, err := getProperActionFunc(extendHTMLProcessorFuncs.actions, action.Method); err == nil {
					if _, err := f(s, unit.Key, data, action.Params); err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
		}
	}
	return nil, nil
}

func (ht *HTMLProcessor) Assert(s *types.Session, body interface{}) error {
	data, ok := body.([]byte)
	if !ok {
		return ErrIncorrectParamType
	}

	for _, unit := range ht.units {
		for _, assert := range unit.Asserts {
			switch assert.Method {
			case "contain":
				if len(assert.Params) <= 0 {
					return ErrMissedRequiredParams
				}
				for _, param := range assert.Params {
					if p, ok := param.(string); ok {
						if !strings.Contains(string(data), p) {
							Logger.Info(fmt.Sprintf("can't find sub string, sub string: %s, org string: %s", p, string(data)))
							return errors.New(fmt.Sprintf("can't find sub string, sub string: %s", p))
						}
					}
				}
			default:
				if f, err := getProperAssertFunc(extendHTMLProcessorFuncs.asserts, assert.Method); err == nil {
					if err := f(s, unit.Key, data, assert.Params); err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}
	}
	return nil
}

func getProperActionFunc(funcs map[ActionType]ActionFunc, m ActionType) (ActionFunc, error) {
	var f ActionFunc
	var ok bool
	if f, ok = funcs[m]; ok {
		return f, nil
	} else if f, ok = extendGlobalProcessorFunc.actions[m]; ok {
		return f, nil
	} else {
		Logger.Debug("unknown proper action", zap.Any("global action", extendGlobalProcessorFunc.actions), zap.Any("action type", m))
		return nil, NewErrUnknownAction(m)
	}
}

func getProperAssertFunc(funcs map[AssertType]AssertFunc, m AssertType) (AssertFunc, error) {
	var f AssertFunc
	var ok bool
	if f, ok = funcs[m]; ok {
		return f, nil
	} else if f, ok = extendGlobalProcessorFunc.asserts[m]; ok {
		return f, nil
	} else {
		return nil, errors.New("unknown assert method: " + string(m))
	}
}

func RegisterGlobalActionFunc(n ActionType, f ActionFunc) {
	extendGlobalProcessorFunc.actions[n] = f
	Logger.Debug("register global action func", zap.Any("action type", n))
}

func RegisterGlobalAssertFunc(n AssertType, f AssertFunc) {
	extendGlobalProcessorFunc.asserts[n] = f
}
