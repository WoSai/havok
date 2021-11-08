package processor

import (
	"github.com/wosai/havok/logger"
	"github.com/wosai/havok/types"
)

type (
	// ActionType 动作类型
	ActionType string

	// Actor 动作单元，负责进行指定的操作
	Actor struct {
		Method ActionType    `json:"action" yaml:"action" toml:"action"`
		Params []interface{} `json:"params,omitempty" yaml:"params,omitempty" toml:"params"`
	}

	// AssertType 断言方法
	AssertType string

	// Assertor 断言单元，负责对指定的key进行断言
	Assertor struct {
		Method AssertType    `json:"assert" yaml:"assert" toml:"assert"`
		Params []interface{} `json:"params,omitempty" yaml:"params,omitempty" toml:"params"`
	}

	// Unit 单元
	Unit struct {
		Key     string      `json:"key" yaml:"key" toml:"key"`
		Actions []*Actor    `json:"actions,omitempty" yaml:"actions,omitempty" toml:"actions"`
		Asserts []*Assertor `json:"asserts,omitempty" yaml:"asserts,omitempty" toml:"asserts"`
	}

	// Units 单元集合
	Units []*Unit

	// ActionFunc 通用Action方法
	ActionFunc func(s *types.Session, key string, data interface{}, params []interface{}) (interface{}, error)

	// AssertFunc 通用的断言方法定义
	AssertFunc func(s *types.Session, key string, data interface{}, params []interface{}) error
)

var (
	Logger = logger.Logger
)
