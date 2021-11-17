package processor

import (
	"errors"
	"fmt"
)

type (
	// AssertionError 断言错误，只有在断言不成功时才返回，在断言前出现err时不会返回该类型
	AssertionError struct {
		expected  interface{}
		actual    interface{}
		astMethod AssertType
		key       string
	}

	// ErrUnknownAction 不支持的Action
	ErrUnknownAction struct {
		action ActionType
	}
)

var (
	// ErrMissedRequiredParams 配置文件中缺少断言参数
	ErrMissedRequiredParams = errors.New("missed required params")
	// ErrIncorrectParamType 配置文件的中的参数类型与期望不一致
	ErrIncorrectParamType = errors.New("incorrect param type")
	// ErrFailedConvertToStringOrNumber 尝试转换成string或者number类型失败
	ErrFailedConvertToStringOrNumber = errors.New("failed to convert to string or number")
	// ErrUnsupportedExpression 断言、action表达式不支持
	ErrUnsupportedExpression = errors.New("unsupported expression")
	// ErrUnexpectedDataType json字段的值类型非期望
	ErrUnexpectedDataType = errors.New("unexpected data type")
	// ErrBadParamValue params的值错误
	ErrBadParamValue = errors.New("bad param value")
	// ErrUnknownAction
	//ErrUnknownAction = errors.New("unknown action")
	// ErrUnknownAssert 不支持的断言方式
	ErrUnknownAssert = errors.New("unknown assert")
)

// NewAssertionError 实例化函数
func NewAssertionError(key string, method AssertType, actual, expected interface{}) *AssertionError {
	return &AssertionError{
		expected:  expected,
		actual:    actual,
		astMethod: method,
		key:       key,
	}
}

// Error AssertionError对error的实现
func (ae *AssertionError) Error() string {
	if ae.expected == nil {
		return fmt.Sprintf("AssertionError on key %s: %v is %s", ae.key, ae.actual, ae.astMethod)
	}
	return fmt.Sprintf("AssertionError on key %s: %v %s %v", ae.key, ae.actual, ae.astMethod, ae.expected)
}

func NewErrUnknownAction(action ActionType) *ErrUnknownAction {
	return &ErrUnknownAction{action: action}
}

func (ua *ErrUnknownAction) Error() string {
	return fmt.Sprintf("unknonw action type: %s", ua.action)
}
