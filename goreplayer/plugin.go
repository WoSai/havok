package replayer

import (
	"errors"
	"net/url"
	"os"
	"plugin"
)

type (
	SelectBuilder interface {
		Select(url *url.URL, header map[string]string, method string, body []byte) string
	}
)

func ConvertAPISelector(b SelectBuilder) APISelector {
	return func(url *url.URL, header map[string]string, method string, body []byte) HTTPAPI {
		return HTTPAPI(b.Select(url, header, method, body))
	}
}

func Load(path string) (APISelector, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	plug, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}
	an, err := plug.Lookup("Selector")
	if err != nil {
		return nil, err
	}
	build, ok := an.(SelectBuilder)
	if !ok {
		return nil, errors.New("plugin not implement interface replayer.SelectBuilder")
	}
	return ConvertAPISelector(build), nil
}
