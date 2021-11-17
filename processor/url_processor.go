package processor

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/wosai/havok/types"
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
)

type URLProcessor struct {
	units Units
}

var (
	extendURLProcessorFuncs = &struct {
		actions map[ActionType]ActionFunc
		asserts map[AssertType]AssertFunc
	}{actions: make(map[ActionType]ActionFunc), asserts: make(map[AssertType]AssertFunc)}
)

// NewURLProcessor URLProcessor的构造函数，需要传入Units
func NewURLProcessor(us Units) *URLProcessor {
	return &URLProcessor{units: us}
}

// WithUnits 更新Units
func (up *URLProcessor) WithUnits(us Units) {
	up.units = us
}

// Act URLProcessor的Action处理函数
func (up *URLProcessor) Act(s *types.Session, u interface{}) (interface{}, error) {
	obj, ok := u.(*url.URL)
	if !ok {
		return nil, errors.New("bad url.URL type")
	}

	for _, unit := range up.units {
		for _, action := range unit.Actions {
			value := up.getField(obj, unit.Key)

			switch action.Method {
			case actionPrint:
				Logger.Info("print key-value", zap.String(unit.Key, value))

			case actionSetValue:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				up.setField(obj, unit.Key, v)

			case actionGetFromSession:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				if val, loaded := s.Get(v); loaded {
					if s, ok := val.(string); ok {
						up.setField(obj, unit.Key, s)
					} else {
						return nil, errors.New(fmt.Sprintf("bad type, expect string, actual: %T", val))
					}
				} else {
					return nil, errors.New(fmt.Sprintf("cannot found item from session with value %s", unit.Key))
				}

			case actionStoreIntoSession:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				s.Put(v, value)

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
				up.setField(obj, unit.Key, string(RandStringBytesMaskImprSrc(length)))

			case actionSetCurrentTimestamp:
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				up.setField(obj, unit.Key, strconv.FormatInt(currentTimestamp(v), 10))

			case actionSetUUID:
				up.setField(obj, unit.Key, uuid.NewV4().String())

			case "reverse_path_params":
				if len(action.Params) != 1 {
					return nil, errors.New("bad params")
				}
				style, ok := action.Params[0].(string)
				if !ok {
					return nil, errors.New("bad restful style")
				}
				if ks, err := reverseRESTPath(style, obj.Path); err == nil {
					up.setField(obj, unit.Key, ks)
				} else {
					return nil, err
				}

			case "parse_path_params":
				v, err := checkIfStringType(action.Params)
				if err != nil {
					return nil, err
				}
				ks, err := parseRESTPath(v, obj.Path)
				if err != nil {
					return nil, err
				}
				for k, v := range ks {
					s.Put(k, v)
				}

			case "set_path_params":
				if len(action.Params) <= 1 {
					return nil, errors.New("bad params")
				}
				style, ok := action.Params[0].(string)
				if !ok {
					return nil, errors.New("bad restful style")
				}
				var es []string
				for _, param := range action.Params[1:] {
					p, ok := param.(string)
					if !ok {
						return nil, errors.New("failed to convert as string")
					}
					es = append(es, p)
				}
				np, err := renderRESTPath(style, es...)
				if err != nil {
					return nil, err
				}
				up.setField(obj, unit.Key, np)

			case "set_path_params_by_session_keys":
				if len(action.Params) <= 1 {
					return nil, errors.New("bad params")
				}
				style, ok := action.Params[0].(string)
				if !ok {
					return nil, errors.New("bad restful style")
				}

				var es []string
				for _, param := range action.Params[1:] {
					p, ok := param.(string)
					if !ok {
						return nil, errors.New("failed to convert as string")
					}
					vv, loaded := s.Get(p)
					if !loaded {
						return nil, errors.New("cannot found item from session with key: " + p)
					}
					vvv, ok := vv.(string)
					if !ok {
						return nil, errors.New(fmt.Sprintf("bad type: %T", vv))
					}
					es = append(es, vvv)
				}
				np, err := renderRESTPath(style, es...)
				if err != nil {
					return nil, err
				}
				up.setField(obj, unit.Key, np)

				//action.Params[style, mask, position]
			case "set_path_params_by_mask":
				if len(action.Params) != 2 {
					return nil, errors.New("bad params")
				}
				style, ok := action.Params[0].(string)
				if !ok {
					return nil, errors.New("bad restful style")
				}
				expression, ok := action.Params[1].(string)
				if !ok {
					return nil, errors.New("bad mask type")
				}
				if ks, err := reverseRESTPathByMask(style, obj.Path, expression); err == nil {
					up.setField(obj, unit.Key, ks)
				} else {
					return nil, err
				}

			default:
				if f, err := getProperActionFunc(extendURLProcessorFuncs.actions, action.Method); err == nil {
					if _, err := f(s, unit.Key, obj, action.Params); err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
		}
	}

	return obj, nil
}

// Assert URLProcessor的断言函数
func (up *URLProcessor) Assert(s *types.Session, u interface{}) error {
	for _, unit := range up.units {
		for _, assert := range unit.Asserts {
			switch assert.Method {

			default:
				if f, err := getProperAssertFunc(extendURLProcessorFuncs.asserts, assert.Method); err == nil {
					if err := f(s, unit.Key, u, assert.Params); err != nil {
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

func (up *URLProcessor) getField(u *url.URL, key string) string {
	switch key {
	case "host":
		return u.Host

	case "path":
		return u.Path

	case "scheme":
		return u.Scheme

	case "query":
		return u.RawQuery

	default:
		return "*"
	}
}

func (up *URLProcessor) setField(u *url.URL, key, value string) {
	switch key {
	case "host":
		u.Host = value

	case "path":
		u.Path = value

	case "scheme":
		u.Scheme = value

	case "query":
		u.RawQuery = value

	default:

	}
}

// RegisterActionFuncInURLProcessor 注册第三方实现的ActionFunc
func RegisterActionFuncInURLProcessor(n ActionType, f ActionFunc) {
	extendURLProcessorFuncs.actions[n] = f
}

// RegisterAssertFuncInURLProcessor 注册第三方实现的AssertFunc
func RegisterAssertFuncInURLProcessor(n AssertType, f AssertFunc) {
	extendURLProcessorFuncs.asserts[n] = f
}

func parseRESTPath(style, path string) (map[string]string, error) {
	ps := strings.Split(path, `/`)
	ss := strings.Split(style, `/`)
	if len(ps) != len(ss) {
		return nil, errors.New("bad rest style")
	}

	ret := make(map[string]string)
	for i, sss := range ss {
		if strings.HasPrefix(sss, "{") && strings.HasSuffix(sss, "}") {
			ret[sss] = ps[i]
		}
	}
	return ret, nil
}

func renderRESTPath(style string, element ...string) (string, error) {
	ss := strings.Split(style, `/`)
	var rs []int

	for index, sss := range ss {
		if strings.HasPrefix(sss, "{") && strings.HasSuffix(sss, "}") {
			rs = append(rs, index)
		}
	}

	if len(rs) != len(element) {
		return "", errors.New("invalid elements")
	}

	for i, e := range element {
		ss[rs[i]] = e
	}
	return strings.Join(ss, "/"), nil
}

func reverseRESTPath(style, path string) (string, error) {
	ps := strings.Split(path, `/`)
	ss := strings.Split(style, `/`)
	if len(ps) != len(ss) {
		return "", errors.New("bad rest style")
	}
	for i, sss := range ss {
		if strings.HasPrefix(sss, "{") && strings.HasSuffix(sss, "}") {
			ps[i] = reverse(ps[i])
		}
	}
	return strings.Join(ps, "/"), nil
}

func reverseRESTPathByMask(style, path, expression string) (string, error) {
	ps := strings.Split(path, `/`)
	ss := strings.Split(style, `/`)
	if len(ps) != len(ss) {
		return "", errors.New("bad rest style")
	}
	for i, sss := range ss {
		if strings.HasPrefix(sss, "{") && strings.HasSuffix(sss, "}") {
			ps[i] = strings.Replace(expression, buildStringPlaceHolder, ps[i], -1)
		}
	}
	return strings.Join(ps, "/"), nil
}
