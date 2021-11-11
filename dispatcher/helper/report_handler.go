package helper

import (
	"fmt"
	"sort"
	"strings"
	"time"

	infx2 "github.com/influxdata/influxdb-client-go/v2"
	infx2api "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/json-iterator/go"
	"github.com/wosai/havok/dispatcher"
	"github.com/wosai/havok/internal/logger"
	"github.com/wosai/havok/internal/option"
	"github.com/wosai/havok/types"
	"go.uber.org/zap"
)

var (
	cutLine = strings.Repeat("-", 126)
)

type (

	// InfluxDBHelper .
	InfluxDBHelper struct {
		client infx2.Client
		conf   option.InfluxDBOption
		writer infx2api.WriteAPI
	}

	// InfluxDBHelperConfig InfluxDBHelper配置
	InfluxDBHelperConfig struct {
		URL                    string
		AuthToken              string
		Organization           string
		Bucket                 string
		Database               string
		MeasurementSucc        string
		MeasurementFail        string
		MeasurementAggregation string
	}

	SLSLogtail struct {
		output string
	}
)

// NewInfluxDBHelper 实例化NewInfluxDBHelper对象
func NewInfluxDBHelper(conf option.InfluxDBOption) (*InfluxDBHelper, error) {
	client := infx2.NewClientWithOptions(conf.ServerURL, conf.AuthToken,
		infx2.DefaultOptions().SetBatchSize(uint(conf.BatchSize)).SetFlushInterval(uint(conf.FlushInterval)).SetPrecision(time.Millisecond))
	// Get non-blocking write client
	noBlockWriteAPI := client.WriteAPI(conf.Organization, conf.Bucket)
	return &InfluxDBHelper{
		client: client,
		conf:   conf,
		writer: noBlockWriteAPI,
	}, nil
}

// HandleReport 处理聚合报告
func (i *InfluxDBHelper) HandleReport() dispatcher.ReportHandleFunc {
	return func(r types.Report, _ types.PerformanceStat) {
		for _, report := range r {
			// 不处理total
			if report.FullHistory {
				return
			}

			point := infx2.NewPoint(
				i.conf.Measurement,
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
			i.writer.WritePoint(point)
		}
	}
}

func (i *InfluxDBHelper) Close() {
	fmt.Println("ihelper close")
	i.writer.Flush()
	i.client.Close()
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
			logger.Logger.Error("marshel report object failed", zap.Error(err))
		}
	}
}

func LogTailFeeder(report types.Report, _ types.PerformanceStat) {
	for _, r := range report {
		logger.Logger.Info("stress test report", zap.String("api", r.Name), zap.Any("report", r))
	}
}
