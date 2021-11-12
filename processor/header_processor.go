package processor

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/wosai/havok/types"
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
)

type (
	// HTTPHeaderProcessor http header的通用处理器
	HTTPHeaderProcessor struct {
		units     Units
		compilers map[string]*regexp.Regexp
	}
)

var (
	extendHeaderProcessFuncs = &struct {
		actions map[ActionType]ActionFunc
		asserts map[AssertType]AssertFunc
	}{actions: map[ActionType]ActionFunc{}, asserts: map[AssertType]AssertFunc{}}
)

// NewHTTPHeaderProcessor HTTPHeaderProcessor构造函数，需要Units,如果默认传入nil,之后可以调用WithUnits重新设置
func NewHTTPHeaderProcessor(us Units) *HTTPHeaderProcessor {
	return &HTTPHeaderProcessor{units: us, compilers: map[string]*regexp.Regexp{}}
}

// WithUnits 更新Units
func (hhp *HTTPHeaderProcessor) WithUnits(us Units) {
	hhp.units = us
}

// Act HTTPHeaderProcessor的Action处理函数
func (hhp *HTTPHeaderProcessor) Act(session *types.Session, h interface{}) (interface{}, error) {
	header, ok := h.(http.Header)
	if !ok {
		return nil, errors.New("bad http header")
	}

	for _, unit := range hhp.units {
		for _, action := range unit.Actions {
			switch action.Method {
			case actionPrint:
				Logger.Info("print key", zap.String(unit.Key, header.Get(unit.Key)))

			case actionSetValue:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				header.Set(unit.Key, v)

			case "add":
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				header.Add(unit.Key, v)

			case actionDelete:
				header.Del(unit.Key)

			case actionGetFromSession:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}

				if val, loaded := session.Get(v); loaded {
					if s, ok := val.(string); ok {
						header.Set(unit.Key, s)
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
				session.Put(v, header.Get(unit.Key))

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
				header.Set(unit.Key, string(RandStringBytesMaskImprSrc(length)))

			case actionSetCurrentTimestamp:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				header.Set(unit.Key, strconv.FormatInt(currentTimestamp(v), 10))

			case actionSetUUID:
				header.Set(unit.Key, uuid.NewV4().String())

			case "regexp_picker":
				if len(action.Params) != 3 {
					return nil, ErrMissedRequiredParams
				}
				expr, ok := action.Params[0].(string)
				if !ok {
					return nil, ErrIncorrectParamType
				}

				index, ok := action.Params[1].(int64)
				if !ok {
					return nil, ErrIncorrectParamType
				}

				k, ok := action.Params[2].(string)
				if !ok {
					return nil, ErrIncorrectParamType
				}
				re, exist := hhp.compilers[expr]
				if !exist {
					c, err := regexp.Compile(expr)
					if err != nil {
						return nil, err
					}
					hhp.compilers[expr] = c
					re = c
				}
				matched := re.FindStringSubmatch(header.Get(unit.Key))[index]
				if matched == "" {
					return nil, errors.New(fmt.Sprintf("no matched string, expr: %s", re.String()))
				}
				session.Put(k, matched)

			default:
				if f, err := getProperActionFunc(extendHeaderProcessFuncs.actions, action.Method); err == nil {
					if _, err := f(session, unit.Key, h, action.Params); err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
		}
	}
	return header, nil
}

// Assert 该接口暂不实现，理论上header不需要断言
func (hhp *HTTPHeaderProcessor) Assert(session *types.Session, data interface{}) error {
	for _, unit := range hhp.units {
		for _, assert := range unit.Asserts {
			switch assert.Method {

			default:
				if f, err := getProperAssertFunc(extendHeaderProcessFuncs.asserts, assert.Method); err == nil {
					if err := f(session, unit.Key, data, assert.Params); err != nil {
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

// RegisterActionFuncInHTTPHeaderProcessor 注册第三方实现的ActionFunc，用于扩展HTTPHeaderProcessor的能力
func RegisterActionFuncInHTTPHeaderProcessor(n ActionType, f ActionFunc) {
	extendHeaderProcessFuncs.actions[n] = f
}

// RegisterAssertFuncInHTTPHeaderProcessor 注册第三方实现的AssertFunc,扩展HTTPHeaderProcessor的断言能力
func RegisterAssertFuncInHTTPHeaderProcessor(n AssertType, f AssertFunc) {
	extendHeaderProcessFuncs.asserts[n] = f
}
