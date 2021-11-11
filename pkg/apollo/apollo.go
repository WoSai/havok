package apollo

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/shima-park/agollo"
	"reflect"
	"strconv"
	"strings"
)

type (
	Option func(*options)

	options struct {
		appId           string // default ""
		namespace       string // default "application"
		configServerURL string // default "beta.apollo.config.shouqianba.com"
	}

	Client struct {
		agollo.Agollo
		opt options
	}

	Env string
)

func WithNamespace(ns string) Option {
	return func(o *options) {
		o.namespace = ns
	}
}

func WithAppId(appId string) Option {
	return func(o *options) {
		o.appId = appId
	}
}

func WithUrl(url string) Option {
	return func(o *options) {
		o.configServerURL = url
	}
}

func NewClient(opts ...Option) (*Client, error) {
	opt := &options{
		appId:           "",
		namespace:       "application",
		configServerURL: "",
	}

	for _, f := range opts {
		f(opt)
	}
	if opt.appId == "" {
		return nil, errors.New("apollo appId cannot be empty.")
	}
	apollo, err := agollo.New(opt.configServerURL, opt.appId, agollo.PreloadNamespaces(opt.namespace))
	if err != nil {
		return nil, fmt.Errorf("failed connecting to apollo. %w", err)
	}
	_ = apollo.Start()
	return &Client{apollo, *opt}, nil
}

func (c *Client) GetConfig(key string) (interface{}, bool) {
	conf, ok := c.GetNameSpace(c.opt.namespace)[key]
	return conf, ok
}

func unmarshalFromApollo(conf map[string]interface{}, opts interface{}) {
	v := reflect.ValueOf(opts).Elem()
	t := reflect.TypeOf(opts).Elem()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, ok := f.Tag.Lookup("json")
		value, ok2 := conf[tag].(string)
		if ok && ok2 {
			switch f.Type.Kind() {
			case reflect.Struct, reflect.Array, reflect.Slice, reflect.Map:
				b := []byte(value)
				_ = json.Unmarshal(b, v.FieldByName(f.Name).Addr().Interface())
			case reflect.String:
				v.FieldByName(f.Name).SetString(value)
			case reflect.Int64:
				intv, err := strconv.Atoi(value)
				if err == nil {
					v.FieldByName(f.Name).SetInt(int64(intv))
				}
			case reflect.Float64:
				floatv, err := strconv.ParseFloat(value, 64)
				if err == nil {
					v.FieldByName(f.Name).SetFloat(floatv)
				}
			case reflect.Bool:
				v.FieldByName(f.Name).SetBool(value == "1" || strings.ToLower(value) == "true")
			}
		}
	}
}
