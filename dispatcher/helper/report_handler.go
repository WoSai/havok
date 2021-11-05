package helper

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/WoSai/havok/dispatcher"
	"github.com/WoSai/havok/types"
	influx "github.com/influxdata/influxdb/client/v2"
	"github.com/json-iterator/go"
	"go.uber.org/zap"
)

var (
	cutLine = strings.Repeat("-", 126)
)

type (
	batchPointsBuffer struct {
		helper   *InfluxDBHelper
		bp       influx.BatchPoints
		interval time.Duration
		mu       sync.Mutex
		conf     influx.BatchPointsConfig
	}

	// InfluxDBHelper .
	InfluxDBHelper struct {
		client influx.Client
		conf   *InfluxDBHelperConfig
		buffer *batchPointsBuffer
	}

	// InfluxDBHelperConfig InfluxDBHelper配置
	InfluxDBHelperConfig struct {
		URL                    string
		UDP                    bool
		User                   string
		Password               string
		Database               string
		MeasurementSucc        string
		MeasurementFail        string
		MeasurementAggregation string
	}

	SLSLogtail struct {
		output string
	}
)

const (
	// DefaultInfluxDBURL influxdb address
	DefaultInfluxDBURL = "http://127.0.0.1:8086"
	// DefaultInfluxDBName influxdb database name
	DefaultInfluxDBName = "havok"
	// DefaultMeasurementSucc the measurement to store successful request
	DefaultMeasurementSucc = "success"
	// DefaultMeasurementFail the measurement to store failed request
	DefaultMeasurementFail = "failures"
	// DefaultMeasurementAggregation the measurement to store report
	DefaultMeasurementAggregation = "report"
)

func (b *batchPointsBuffer) addPoint(p *influx.Point) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.bp == nil {
		bp, err := influx.NewBatchPoints(b.conf)
		if err != nil {
			dispatcher.Logger.Error("failed to call NewBatchPoints: " + err.Error())
			return
		}
		b.bp = bp
	}
	b.bp.AddPoint(p)
}

func (b *batchPointsBuffer) flushing() {
	for {
		time.Sleep(b.interval)

		b.mu.Lock()
		if b.bp == nil {
			b.mu.Unlock()
			continue
		}

		go func(c influx.Client, bp influx.BatchPoints) {
			err := c.Write(bp)
			if err != nil {
				dispatcher.Logger.Error("failed to write batch points: " + err.Error())
			} else {
				dispatcher.Logger.Info("succeed to write batch points into influxdb")
			}
		}(b.helper.client, b.bp)

		b.bp = nil
		b.mu.Unlock()
	}

}

func newInfluxDBHTTPClient(url, user, password string) (influx.Client, error) {
	return influx.NewHTTPClient(influx.HTTPConfig{
		Addr:     url,
		Username: user,
		Password: password,
	})
}

func newInfluxDBUDPClient(url string) (influx.Client, error) {
	return influx.NewUDPClient(influx.UDPConfig{
		Addr: url,
	})
}

// NewInfluxDBHelperConfig 实例化InfluxDBHelpConfig默认配置
func NewInfluxDBHelperConfig() *InfluxDBHelperConfig {
	return &InfluxDBHelperConfig{
		URL:                    DefaultInfluxDBURL,
		UDP:                    false,
		User:                   "",
		Password:               "",
		Database:               DefaultInfluxDBName,
		MeasurementSucc:        DefaultMeasurementSucc,
		MeasurementFail:        DefaultMeasurementFail,
		MeasurementAggregation: DefaultMeasurementAggregation,
	}
}

// NewInfluxDBHelper 实例化NewInfluxDBHelper对象
func NewInfluxDBHelper(conf *InfluxDBHelperConfig) (*InfluxDBHelper, error) {
	var err error
	var client influx.Client
	if conf.UDP {
		client, err = newInfluxDBUDPClient(conf.URL)
	} else {
		client, err = newInfluxDBHTTPClient(conf.URL, conf.User, conf.Password)
	}

	if err != nil {
		dispatcher.Logger.Error("failed to init influxdb client: " + err.Error())
		return nil, err
	}

	buf := &batchPointsBuffer{
		helper:   nil,
		interval: time.Millisecond * 500,
		conf:     influx.BatchPointsConfig{Precision: "ms", Database: conf.Database},
	}
	helper := &InfluxDBHelper{client: client, conf: conf, buffer: buf}
	buf.helper = helper
	go buf.flushing()

	return helper, nil
}

// HandleReport 处理聚合报告
func (i *InfluxDBHelper) HandleReport() dispatcher.ReportHandleFunc {
	return func(r types.Report, _ types.PerformanceStat) {
		for _, report := range r {
			// 不处理total
			if report.FullHistory {
				return
			}

			point, err := influx.NewPoint(
				i.conf.MeasurementAggregation,
				map[string]string{"api": report.Name},
				map[string]interface{}{
					"qps":        report.QPS,
					"success":    report.Requests,
					"failures":   report.Failures,
					"fail_ratio": report.FailRatio,
					"min":        report.Min,
					"max":        report.Max,
					"avg":        report.Average,
					"TP50":       report.Distributions["0.50"],
					"TP60":       report.Distributions["0.60"],
					"TP70":       report.Distributions["0.70"],
					"TP80":       report.Distributions["0.80"],
					"TP90":       report.Distributions["0.90"],
					"TP95":       report.Distributions["0.95"],
					"TP96":       report.Distributions["0.96"],
					"TP97":       report.Distributions["0.97"],
					"TP98":       report.Distributions["0.98"],
					"TP99":       report.Distributions["0.99"],
				},
				time.Now(),
			)
			if err != nil {
				dispatcher.Logger.Error("failed to create new point: " + err.Error())
			} else {
				i.buffer.addPoint(point)
			}
		}
	}
}

func PrintReportToConsole(report types.Report, perfStat types.PerformanceStat) {
	var full bool
	var keys []string
	//var f *os.File
	//var err error

	for k, r := range report {
		keys = append(keys, k)
		if r.FullHistory {
			full = r.FullHistory
		}
	}
	sort.Strings(keys)

	s := fmt.Sprintf("|%-48s|%12s|%12s|%12s|%8s|%9s|%8s|%8s|\n", "Name", "Requests", "Failures", "QPS", "Min", "Max", "Avg", "Median")
	d := fmt.Sprintf("\nPercentage of the requests completed within given times: \n\n|%-48s|%12s|%8s|%8s|%8s|%8s|%8s|%8s|%8s|\n", "Name", "Requests", "60%", "70%", "80%", "90%", "95%", "98%", "99%")
	for _, key := range keys {
		r := report[key]
		s += fmt.Sprintf("|%-48s|%12d|%12d|%12d|%8d|%9d|%8d|%8d|\n", r.Name, r.Requests, r.Failures, r.QPS, r.Min, r.Max, r.Average, r.Median)
		d += fmt.Sprintf("|%-48s|%12d|%8d|%8d|%8d|%8d|%8d|%8d|%8d|\n", r.Name, r.Requests, r.Distributions["0.60"], r.Distributions["0.70"], r.Distributions["0.80"], r.Distributions["0.90"], r.Distributions["0.95"], r.Distributions["0.98"], r.Distributions["0.99"])
	}
	op := cutLine + "\n" + s + d + cutLine + "\n"
	fmt.Println(op)
	fmt.Println(perfStat.Stats)
	//if additionalOutput != "" {
	//	f, err = os.OpenFile(additionalOutput, os.O_APPEND|os.O_WRONLY, 0666)
	//	if err == nil {
	//		defer f.Close()
	//		f.WriteString(op)
	//	}
	//}

	if full {
		data, err := jsoniter.MarshalIndent(report, "", "  ")
		if err == nil {
			op = "============= Summary Report =============\n\n" + string(data) + "\n"
			fmt.Printf(op)
			//if additionalOutput != "" {
			//	f.WriteString(op)
			//}
		} else {
			dispatcher.Logger.Error("marshel report object failed", zap.Error(err))
		}
	}
}

func LogTailFeeder(report types.Report, _ types.PerformanceStat) {
	for _, r := range report {
		dispatcher.Logger.Info("stress test report", zap.String("api", r.Name), zap.Any("report", r))
	}
}
