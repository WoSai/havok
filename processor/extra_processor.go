package processor

import (
	"github.com/wosai/havok/types"
	"go.uber.org/zap"
)

type (
	ExtraProcessor struct {
		units Units
	}
)

var (
	extendExtraProcessorFuncs = &struct {
		actions map[ActionType]ActionFunc
		asserts map[AssertType]AssertFunc
	}{actions: make(map[ActionType]ActionFunc), asserts: make(map[AssertType]AssertFunc)}
)

func NewExtraProcssor(us Units) *ExtraProcessor {
	return &ExtraProcessor{units: us}
}

func (ep *ExtraProcessor) WithUnits(us Units) {
	ep.units = us
}

//手动处理额外的send事件, 返回[]map[ActionType][]interface{}类型，其中[]interface{}类型理论上都应该为[]*dispatcher.LogRecord{}类型
func (ep *ExtraProcessor) Act(s *types.Session, data interface{}) (interface{}, error) {
	ss := make([]map[string][]interface{}, 0)
	for _, unit := range ep.units {
		for _, action := range unit.Actions {
			switch action.Method {

			default:
				if f, err := getProperActionFunc(extendExtraProcessorFuncs.actions, action.Method); err == nil {
					if ret, err := f(s, unit.Key, data, action.Params); err != nil {
						return nil, err
					} else {
						var flag bool
						for _, temp := range ss {
							if d, ok := temp[string(action.Method)]; ok {
								temp[string(action.Method)] = append(d, ret)
								flag = true
							}
						}
						if !flag {
							temp := make(map[string][]interface{})
							temp[string(action.Method)] = []interface{}{ret}
							ss = append(ss, temp)
						}
					}
				} else {
					return nil, err
				}
			}
		}
	}
	Logger.Info("extra act", zap.Any("act", ss))
	return ss, nil
}

//理论上不会用到这个场景
func (ep *ExtraProcessor) Assert(s *types.Session, data interface{}) error {
	for _, unit := range ep.units {
		for _, assert := range unit.Asserts {
			switch assert.Method {

			default:
				if f, err := getProperAssertFunc(extendExtraProcessorFuncs.asserts, assert.Method); err == nil {
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

func RegisterActionFuncInExtraProcessor(n ActionType, f ActionFunc) {
	extendExtraProcessorFuncs.actions[n] = f
}

func RegisterAssertFuncInExtraProcessor(n AssertType, f AssertFunc) {
	extendExtraProcessorFuncs.asserts[n] = f
}
