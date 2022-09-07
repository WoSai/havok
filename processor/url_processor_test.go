package processor

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wosai/havok/types"
)

var (
	Host      = "api.github.com"
	newOrg    = "shouqianba"
	newAction = "query"

	urlUnits = []*Unit{
		{
			Key:     "host",
			Actions: []*Actor{{"set", []interface{}{Host}}},
		},
		{
			Key:     "scheme",
			Actions: []*Actor{{"set", []interface{}{"https"}}},
		},
		{
			Key:     "*",
			Actions: []*Actor{{"store_into_session", []interface{}{"hello"}}},
		},
		{
			Key: "path",
			Actions: []*Actor{
				{"set", []interface{}{"/orgs/Wosai/actions/permissions"}},
				{"parse_path_params", []interface{}{"/upay/{org}/actions/{action}"}},
				{"set_path_params_by_session_keys", []interface{}{"/newpath/orgs/{org}/actions/{action}", "org", "action"}},
			},
		},
	}
)

func TestParsePath(t *testing.T) {
	u, err := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	assert.Nil(t, err)

	processor := NewURLProcessor(urlUnits)
	session := types.NewSession()
	session.Put("org", newOrg)
	session.Put("action", newAction)
	_, err = processor.Act(session, u)
	assert.Nil(t, err)

	assert.Equal(t, u.Scheme, "https")
	assert.Equal(t, u.Host, Host)
	v, exist := session.Get("hello")
	assert.True(t, exist)
	vv := v.(string)
	assert.Equal(t, vv, "*")

	version, _ := session.Get("{org}")
	assert.Equal(t, version, "Wosai")
	action, _ := session.Get("{action}")
	assert.Equal(t, action, "permissions")

	assert.Equal(t, u.Path, fmt.Sprintf("/newpath/orgs/%s/actions/%s", newOrg, newAction))
}

func TestRegisterActionFuncInURLProcessor(t *testing.T) {
	// before register
	urlUnits = append(urlUnits, &Unit{Key: "*", Actions: []*Actor{{"change_item", []interface{}{"key", "value"}}}})
	processor := NewURLProcessor(urlUnits)
	session := types.NewSession()
	session.Put("org", newOrg)
	session.Put("action", newAction)
	u, err := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	assert.Nil(t, err)
	_, err = processor.Act(session, u)
	assert.NotNil(t, err)

	// after register
	RegisterActionFuncInURLProcessor("change_item", func(s *types.Session, _ string, data interface{}, params []interface{}) (i interface{}, e error) {
		key := params[0].(string)
		value := params[1].(string)

		s.Put(key, value)
		return data, nil
	})
	_, err = processor.Act(session, u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActPrintHost(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	var units = []*Unit{
		{
			Key: "host", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActPrintPath(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActPrintScheme(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	var units = []*Unit{
		{
			Key: "scheme", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActPrintQuery(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "query", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActSetHost(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	var units = []*Unit{
		{
			Key: "host", Actions: []*Actor{{"set", []interface{}{"api.wosai.github.com"}}},
		},
		{
			Key: "host", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActSetPath(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"set", []interface{}{"orgs/shouqianba/actions/permissions"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActSetScheme(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	var units = []*Unit{
		{
			Key: "scheme", Actions: []*Actor{{"set", []interface{}{"https"}}},
		},
		{
			Key: "scheme", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActSetQuery(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	var units = []*Unit{
		{
			Key: "query", Actions: []*Actor{{"set", []interface{}{"three=3&four=4"}}},
		},
		{
			Key: "query", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActGetFromSession(t *testing.T) {
	s := types.NewSession()
	s.Put("cot-path", "/upay/v3/pay")

	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"get_from_session", []interface{}{"cot-path"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(s, u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActStoreToSession(t *testing.T) {
	s := types.NewSession()
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "query", Actions: []*Actor{{"store_into_session", []interface{}{"cot-path"}}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(s, u)
	assert.Nil(t, err)

	val, f := s.Get("cot-path")
	assert.True(t, f)
	assert.Equal(t, "one=1&two=2", val.(string))
}

func TestURLProcessor_ActRandom(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"rand_string", []interface{}{8}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActTimestamp(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"current_timestamp", []interface{}{"ms"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActUUID(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"uuid", []interface{}{"ms"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActReversePathParams0(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"reverse_path_params", []interface{}{"/orgs/{org}/actions/permissions"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActReversePathParams1(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"reverse_path_params", []interface{}{"/orgs/{org}"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Error(t, err)
}

func TestURLProcessor_ActParserPathParams(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"parse_path_params", []interface{}{"/orgs/{org}/actions/permissions"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	s := types.NewSession()
	processor := NewURLProcessor(units)
	_, err := processor.Act(s, u)
	assert.Nil(t, err)
	val, f := s.Get("{org}")
	assert.True(t, f)
	assert.Equal(t, "Wosai", val.(string))
}

func TestURLProcessor_ActSetPathParams0(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"set_path_params", []interface{}{"/orgs/{org}/actions/{action}", newOrg, newAction}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActSetPathParams1(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"set_path_params", []interface{}{"/orgs/{org}/actions/{action}", newOrg}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Error(t, err)
}

func TestURLProcessor_ActSetPathParamsBySessionKey0(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"set_path_params_by_session_keys", []interface{}{"/orgs/{org}/actions/{action}", "o", "a"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	s := types.NewSession()
	s.Put("o", newOrg)
	s.Put("a", newAction)
	processor := NewURLProcessor(units)
	_, err := processor.Act(s, u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActSetPathParamsBySessionKey1(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"set_path_params_by_session_keys", []interface{}{"/orgs/{org}/actions/{action}", "o", "a"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	s := types.NewSession()
	s.Put("o", newOrg)
	processor := NewURLProcessor(units)
	_, err := processor.Act(s, u)
	assert.Error(t, err)
}

func TestURLProcessor_ActSetPathParamsBySessionKey2(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"set_path_params_by_session_keys", []interface{}{"/orgs/{org}/actions/{action}", "o"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	s := types.NewSession()
	s.Put("o", newOrg)
	processor := NewURLProcessor(units)
	_, err := processor.Act(s, u)
	assert.Error(t, err)
}

func TestURLProcessor_ActSetPathParamsByMask0(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"set_path_params_by_mask", []interface{}{"/orgs/{org}/actions/{action}", "fake_$0"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestURLProcessor_ActSetPathParamsByMask1(t *testing.T) {
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions?one=1&two=2")
	var units = []*Unit{
		{
			Key: "path", Actions: []*Actor{{"set_path_params_by_mask", []interface{}{"/orgs/{org}/actions/{action}", "$0_fake"}}},
		},
		{
			Key: "path", Actions: []*Actor{{"print", nil}},
		},
	}
	processor := NewURLProcessor(units)
	_, err := processor.Act(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestRegisterAssertFuncInURLProcessor0(t *testing.T) {
	RegisterAssertFuncInURLProcessor("xyz", func(s *types.Session, key string, data interface{}, params []interface{}) error {
		return nil
	})
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	var units = []*Unit{
		{
			Key: "host", Asserts: []*Assertor{{"xyz", nil}},
		},
	}
	processor := NewURLProcessor(units)
	err := processor.Assert(types.NewSession(), u)
	assert.Nil(t, err)
}

func TestRegisterAssertFuncInURLProcessor1(t *testing.T) {
	RegisterAssertFuncInURLProcessor("xyz", func(s *types.Session, key string, data interface{}, params []interface{}) error {
		return errors.New("make error")
	})
	u, _ := url.Parse("http://api.github.com/orgs/Wosai/actions/permissions")
	var units = []*Unit{
		{
			Key: "host", Asserts: []*Assertor{{"xyz", nil}},
		},
	}
	processor := NewURLProcessor(units)
	err := processor.Assert(types.NewSession(), u)
	assert.Error(t, err)
}
