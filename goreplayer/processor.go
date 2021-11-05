package replayer

import (
	"io/ioutil"

	"github.com/WoSai/havok/processor"
)

type (
	HTTPAPI string
	FlowScope string

	processConfigBlock struct {
		Type   string          `json:"type,omitempty"`
		Header processor.Units `json:"header,omitempty"`
		URL    processor.Units `json:"url,omitempty"`
		Params processor.Units `json:"params,omitempty"`
		Body   processor.Units `json:"body,omitempty"`
		Extra  processor.Units `json:"extra,omitempty"`
	}

	processorBlock struct {
		header processor.Processor
		url    processor.Processor
		params processor.Processor
		body   processor.Processor
		extra  processor.Processor
	}

	ProcessConfig map[HTTPAPI]*processConfigFlow

	processConfigFlow struct {
		Request  *processConfigBlock `json:"request,omitempty"`
		Response *processConfigBlock `json:"response,omitempty"`
	}

	processorFlow struct {
		request  *processorBlock
		response *processorBlock
	}

	ProcessorHub struct {
		config ProcessConfig
		ps     map[HTTPAPI]*processorFlow
	}
)

func NewProcessorConfigFromFile(fp string) (ProcessConfig, error) {
	data, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, err
	}
	var conf ProcessConfig
	err = json.Unmarshal(data, &conf)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func (pc ProcessConfig) Build() *ProcessorHub {
	ph := &ProcessorHub{
		config: pc,
		ps:     make(map[HTTPAPI]*processorFlow)}

	for api, pf := range pc {
		pb := new(processorFlow)

		b := pf.Request.build()
		if b != nil {
			pb.request = b
		}

		b = pf.Response.build()
		if b != nil {
			pb.response = b
		}

		if pb != nil {
			ph.ps[api] = pb
		}
	}

	return ph
}

func (pb *processConfigBlock) build() *processorBlock {
	if pb == nil {
		return nil
	}

	b := new(processorBlock)

	if pb.Header != nil {
		b.header = processor.NewHTTPHeaderProcessor(pb.Header)
	}

	if pb.URL != nil {
		b.url = processor.NewURLProcessor(pb.URL)
	}

	if pb.Params != nil {
		b.params = processor.NewURLQueryProcessor(pb.Params)
	}

	if pb.Body != nil {
		switch pb.Type {
		case "json":
			b.body = processor.NewJSONProcessor(pb.Body)

		case "form":
			b.body = processor.NewFormBodyProcessor(pb.Body)

		case "html":
			b.body = processor.NewHTMLProcessor(pb.Body)

		}
	}
	if pb.Extra != nil {
		b.extra = processor.NewExtraProcssor(pb.Extra)
	}

	return b
}

func (hh ProcessorHub) GetProcessor(api HTTPAPI, scope FlowScope) *processorBlock {
	if hh.config == nil {
		return nil
	}
	var pf *processorFlow
	var exist bool

	pf, exist = hh.ps[api]
	if !exist {
		pf, exist = hh.ps["default"]
		if !exist {
			return nil
		}
	}

	switch scope {
	case "request":
		return pf.request
	case "response":
		return pf.response
	default:
		return nil
	}
}
