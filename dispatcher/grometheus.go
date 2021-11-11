package dispatcher

import (
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wosai/havok/types"
	"net/http"
	"reflect"
	"sync"
)

type (
	Metrics struct {
		Namespace   string
		Input       chan interface{}
		Analyzer    MetricsAnalyzer
		Selector    MetricSelector
		StoreCenter map[string]interface{}
		Lock        sync.Mutex
	}
	MetricsAnalyzer func(namespace string, store map[string]interface{}, ch chan<- prometheus.Metric)
	MetricSelector  func(d interface{}) string
)

var (
	ProInput = make(chan interface{}, 1000)
)

func NewDesc(namespace, metricName, name, helper string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(namespace+metricName+name, helper, labels, nil)
}

func NewMetrics(namespace string, fa MetricsAnalyzer, fs MetricSelector, input chan interface{}) *Metrics {
	return &Metrics{Namespace: namespace, Analyzer: fa, Selector: fs, Input: input}
}

func (c *Metrics) HandlerFun() http.HandlerFunc {
	registry := prometheus.NewRegistry()
	registry.MustRegister(c)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP
}

func (c *Metrics) Provide() []ProviderMethod {
	return []ProviderMethod{
		{Path: "/metrics",
			Func: c.HandlerFun(),
		},
	}
}

func (c *Metrics) Describe(ch chan<- *prometheus.Desc) {
	go c.fetch()
}

func (c *Metrics) Collect(ch chan<- prometheus.Metric) {
	c.Analyzer(c.Namespace, c.StoreCenter, ch)
}

func (c *Metrics) fetch() {
	for d := range c.Input {
		name := c.Selector(d)
		c.store(name, d)
	}
}

func (c *Metrics) store(name string, d interface{}) {
	c.Lock.Lock()
	defer c.Lock.Unlock()
	if c.StoreCenter == nil {
		m := make(map[string]interface{})
		m[name] = d
		c.StoreCenter = m
	} else {
		c.StoreCenter[name] = d
	}
}

func DefaultSelector(d interface{}) string {
	return reflect.TypeOf(d).Name()
}

func TryGetFloat64(d interface{}) float64 {
	v := reflect.ValueOf(d)
	floatType := reflect.TypeOf(float64(0))
	if !v.Type().ConvertibleTo(floatType) {
		return 0
	}
	return v.Convert(floatType).Float()
}

func HavokAnalyzer(namespace string, store map[string]interface{}, ch chan<- prometheus.Metric) {
	//自定义的storeCenter数据解析，目前是根据不同数据类型做不同处理
	for name, d := range store {
		switch d.(type) {
		case types.Report:
			reports := d.(types.Report)
			desc := NewDesc(namespace, "_", name, "dispatcher summary statistic for havok", []string{"api"})
			for _, r := range reports {
				trend := getTrendFromAttacker(r)
				ch <- prometheus.MustNewConstSummary(desc, uint64(r.Requests), float64(r.Requests+r.Failures), trend, r.Name)
			}
		case types.PerformanceStat:
			perf := d.(types.PerformanceStat)
			desc := NewDesc(namespace, "_", name, "performance stats for havok_replayer", []string{"replay_id", "index"})
			for replayerId, stats := range perf.Stats {
				for k, v := range stats {
					ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, v, replayerId, k)
				}
			}
		default:
			t := make(map[string]interface{})
			desc := NewDesc(namespace, "_", name, "struct field value for havok", []string{"field"})
			ds, _ := json.Marshal(d)
			if err := json.Unmarshal(ds, &t); err == nil {
				for k, v := range t {
					ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, TryGetFloat64(v), k)
				}
			}
		}
	}
}

func getTrendFromAttacker(attacker *types.AttackerReport) map[float64]float64 {
	trend := make(map[float64]float64)
	//自定义约定字段(就先用summary，不想再额外创建自定义metric)
	trend[10] = float64(attacker.QPS)        //QPS标柱为10
	trend[-100] = float64(attacker.Failures) //失败标注为-100
	trend[-1] = float64(attacker.Min)        //最小值约定为-1
	trend[0] = float64(attacker.Average)     //平均值约定为0
	trend[1] = float64(attacker.Max)         //最大值约定为1
	trend[0.5] = float64(attacker.Distributions["0.50"])
	trend[0.6] = float64(attacker.Distributions["0.60"])
	trend[0.7] = float64(attacker.Distributions["0.70"])
	trend[0.8] = float64(attacker.Distributions["0.80"])
	trend[0.9] = float64(attacker.Distributions["0.90"])
	trend[0.95] = float64(attacker.Distributions["0.95"])
	trend[0.96] = float64(attacker.Distributions["0.96"])
	trend[0.97] = float64(attacker.Distributions["0.97"])
	trend[0.98] = float64(attacker.Distributions["0.98"])
	trend[0.99] = float64(attacker.Distributions["0.99"])
	return trend
}

func GrometheusFeed(report types.Report, perfStat types.PerformanceStat) {
	ProInput <- report
	ProInput <- perfStat
}
