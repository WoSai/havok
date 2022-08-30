package dispatcher

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"sync/atomic"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/segmentio/kafka-go"
	pb "github.com/wosai/havok/protobuf"
	"go.uber.org/zap"
	"gopkg.in/olivere/elastic.v5"
)

type (
	// fetcher 定义日志收集者对象
	Fetcher interface {
		Read(chan<- *LogRecordWrapper)
	}

	Decoder interface {
		Decode([]byte) (*LogRecordWrapper, error)
	}

	// ElasticFetcher ElasticSearch的日志收集对象
	ElasticFetcher struct {
		client *elastic.Client
	}

	// FileFetcher 本地日志文件收集者
	FileFetcher struct {
		path   string
		status TaskStatus
		*baseFetcher
	}

	// MultipleFilesFetcher 多文件日志收集对象
	MultipleFilesFetcher struct {
	}

	KafkaSinglePartitionFetcher struct {
		*baseFetcher
		reader  *kafka.Reader
		counter int64
		qps     int64
	}

	// LogRecordWrapper LogRecord的扩展，增加日志时间、哈希字段两个字段
	LogRecordWrapper struct {
		HashField string
		OccurAt   time.Time
		*pb.LogRecord
	}

	baseFetcher struct {
		begin    time.Time
		end      time.Time
		analyzer Analyzer
		output   chan<- *LogRecordWrapper
		parent   ParentTask
		status   TaskStatus
	}

	AliyunSLSConcurrencyFetcher struct {
		store        *sls.LogStore
		AccessKey    string
		AccessSecret string
		Region       string
		Project      string
		LogStore     string
		Expression   string
		PageSize     int64
		cursor       time.Time
		step         time.Duration
		concurrency  int
		preDownload  int
		antsNest     chan *AliyunSLSAnt
		count        int64
		qps          int64
		*baseFetcher
	}

	AliyunSLSAnt struct {
		from        int64
		end         int64
		preDownload int
		output      chan *LogRecordWrapper
		queen       *AliyunSLSConcurrencyFetcher
	}
)

func newBaseFetcher() *baseFetcher {
	return &baseFetcher{}
}

// TimeRange 设定读去日志的时间范围
func (bf *baseFetcher) TimeRange(begin, end time.Time) {
	bf.begin = begin
	bf.end = end
}

// WithAnalyzer 传入日志分析器
func (bf *baseFetcher) WithAnalyzer(a Analyzer) {
	bf.analyzer = a
}

// SetOutput 设定LogRecordWrapper的输出管道
func (bf *baseFetcher) SetOutput(c chan<- *LogRecordWrapper) {
	bf.output = c
}

// Parent 关联任务
func (bf *baseFetcher) Parent(pt ParentTask) {
	bf.parent = pt
}

func (bf *baseFetcher) start() {
	atomic.StoreInt32(&bf.status, StatusRunning)
}

func (bf *baseFetcher) Stop() {
	atomic.CompareAndSwapInt32(&bf.status, StatusRunning, StatusStopped)
	if bf.output != nil {
		close(bf.output)
	}
}

func (bf *baseFetcher) Finish() {
	Logger.Info("file fetcher finished")
	atomic.CompareAndSwapInt32(&bf.status, StatusRunning, StatusFinished)
	if bf.output != nil {
		close(bf.output) // 关闭通道
	}
}

func (bf *baseFetcher) Status() TaskStatus {
	return atomic.LoadInt32(&bf.status)
}

// NewFileFetcher FileFetcher的构造函数
func NewFileFetcher(path string) *FileFetcher {
	return &FileFetcher{path: path, baseFetcher: newBaseFetcher()}
}

// Start 读取日志的文件的每一行，解析出LogRecordWrapper对象，交由TimeWheel按时间顺序分发
func (ff *FileFetcher) Start() error {
	ff.baseFetcher.start()
	if ff.parent != nil {
		ff.parent.Notify(ff, StatusRunning)
	}
	file, err := os.Open(ff.path)
	if err != nil {
		Logger.Error("failed to open file, stop FileFetcher", zap.Error(err))
		ff.Stop()
		if ff.parent != nil {
			ff.parent.Notify(ff, StatusStopped)
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if atomic.LoadInt32(&ff.status) == StatusStopped {
			return ErrTaskInterrupted
		}

		log := ff.analyzer.Analyze(scanner.Bytes())
		if log == nil {
			continue
		}

		if log.OccurAt.After(ff.end) { // 日志时间超出，退出循环
			Logger.Info("time of log is later than end time", zap.String("occurAt", log.OccurAt.String()),
				zap.String("end", ff.end.String()))
			break
		}

		if !log.OccurAt.Before(ff.begin) {
			ff.output <- log
		}
	}

	if err = scanner.Err(); err != nil {
		Logger.Error("failed to load file content, stop FileFetcher", zap.Error(err))
		ff.Stop()
		return err
	}
	Logger.Info("finished to fetcher file")
	ff.Finish()
	return nil
}

func (ff *FileFetcher) Finish() {
	ff.baseFetcher.Finish()
	if ff.parent != nil {
		ff.parent.Notify(ff, StatusFinished)
	}
}

func (ff *FileFetcher) Stop() {
	ff.baseFetcher.Stop()
	if ff.parent != nil {
		ff.parent.Notify(ff, StatusStopped)
	}
}

func NewAliyunSLSAnt(scf *AliyunSLSConcurrencyFetcher, pre int, from, end int64) *AliyunSLSAnt {
	return &AliyunSLSAnt{
		from:        from,
		end:         end,
		preDownload: pre,
		output:      make(chan *LogRecordWrapper, pre),
		queen:       scf,
	}
}

func (sa *AliyunSLSAnt) work() {
	var offset int64 = 0
	for {
		var logs *sls.GetLogsResponse
		var err error
		var retry int

		for ; retry < 3; retry++ {
			logs, err = sa.queen.store.GetLogs("", sa.from, sa.end, sa.queen.Expression, sa.queen.PageSize, offset*sa.queen.PageSize, false)
			if err != nil {
				Logger.Warn("failed to invoke GetLogs, retry again", zap.Error(err))
			} else {
				break
			}
		}
		offset++
		if retry == 3 && err != nil {
			Logger.Info("failed to fetch logs in time range", zap.Int64("from", sa.from), zap.Int64("to", sa.end), zap.Int64("offset", offset))
			continue
		}

		if logs.Count > 0 && sa.queen.analyzer != nil {
			for _, log := range logs.Logs {
				data, _ := json.Marshal(log)
				record := sa.queen.analyzer.Analyze(data)
				if record != nil && sa.output != nil {
					if record.OccurAt.Unix() < sa.from || record.OccurAt.Unix() >= sa.end {
						record.OccurAt = time.Unix(sa.from, rand.Int63n(1000)*1e6) // sls查询结果中的日志会超出时间范围
					}
					sa.output <- record
				}
			}
		}

		if logs.Count < sa.queen.PageSize { // 最后一页
			Logger.Info("reached last page", zap.Int64("begin", sa.from), zap.Int64("end", sa.end))
			break
		}
	}
	close(sa.output)
}

func NewAliyunSLSConcurrencyFetcher(key, secret, region, project, logstore, exp string, concurrency, pre int) (*AliyunSLSConcurrencyFetcher, error) {
	client := &sls.Client{
		Endpoint:        region,
		AccessKeyID:     key,
		AccessKeySecret: secret,
		UserAgent:       "Havok AliyunSLSFetcher",
	}
	sls.GlobalForceUsingHTTP = true
	store, err := client.GetLogStore(project, logstore)
	if err != nil {
		return nil, err
	}
	Logger.Info("sls server was connected")
	return &AliyunSLSConcurrencyFetcher{
			store:        store,
			AccessKey:    key,
			AccessSecret: key,
			Region:       region,
			Project:      project,
			LogStore:     logstore,
			Expression:   exp,
			PageSize:     100,
			step:         1 * time.Second, // sls日志精度为秒级，导致了查询结果无法精确排序，需要手动排序
			concurrency:  concurrency,
			preDownload:  pre,
			antsNest:     make(chan *AliyunSLSAnt, concurrency),
			baseFetcher:  newBaseFetcher(),
		},
		nil
}

// next sls的查询精度为秒，需要特殊处理
func (scf *AliyunSLSConcurrencyFetcher) next() (int64, int64, bool) {
	if scf.cursor.IsZero() {
		scf.cursor = scf.baseFetcher.begin
	}
	begin := scf.cursor
	if !begin.Before(scf.end) {
		return 0, 0, false
	}
	end := begin.Add(scf.step)
	if end.After(scf.end) {
		end = scf.end
		return begin.Unix(), end.Unix(), true
	}
	scf.cursor = end
	return begin.Unix(), end.Unix(), true
}

func (scf *AliyunSLSConcurrencyFetcher) Start() error {
	scf.start()
	if scf.parent != nil {
		scf.parent.Notify(scf, StatusRunning)
	}

	go func() {
		var last = scf.count
		var current int64
		for {
			time.Sleep(1 * time.Second)
			current = atomic.LoadInt64(&scf.count)
			scf.qps = current - last
			last = current
			Logger.Info("AliyunSLSConcurrencyFetcher QPS", zap.Int64("fetcher_qps", scf.qps))
		}
	}()

	go func() { // 切割时间块，开启额外的线程去读取sls日志
		for {
			from, end, ok := scf.next()
			if !ok {
				break
			}

			ant := NewAliyunSLSAnt(scf, scf.preDownload, from, end)
			go ant.work()
			scf.antsNest <- ant
		}
		close(scf.antsNest)
	}()

	var t time.Time
	for ant := range scf.antsNest {
		var reorder []*LogRecordWrapper
		for record := range ant.output {
			//if scf.output != nil {
			//	if t.IsZero() {
			//		t = record.OccurAt
			//	}
			//	if record.OccurAt.Before(t) {
			//		Logger.Warn("out-of-order", zap.Time("log_time", record.OccurAt), zap.Time("last_time", t))
			//	} else {
			//		t = record.OccurAt
			//	}
			//
			//	scf.output <- record
			reorder = append(reorder, record)
		}

		// sls查询结果无法保序，重排序一次
		sort.Slice(reorder, func(i, j int) bool {
			return reorder[i].OccurAt.Before(reorder[j].OccurAt)
		})

		for _, r := range reorder {
			if scf.output != nil {
				if t.IsZero() {
					t = r.OccurAt
				}
				if r.OccurAt.Before(t) {
					Logger.Warn("out-of-order", zap.Time("log_time", r.OccurAt), zap.Time("last_time", t))
				} else {
					t = r.OccurAt
				}
				atomic.AddInt64(&scf.count, 1)
				scf.output <- r
			}
		}

	}

	scf.Finish()
	return nil
}

func (scf *AliyunSLSConcurrencyFetcher) Finish() {
	scf.baseFetcher.Finish()
	if scf.parent != nil {
		scf.parent.Notify(scf, StatusFinished)
	}
}

func (scf *AliyunSLSConcurrencyFetcher) Stop() {
	scf.baseFetcher.Stop()
	if scf.parent != nil {
		scf.parent.Notify(scf, StatusStopped)
	}
}

func (scf *AliyunSLSConcurrencyFetcher) Provide() []ProviderMethod {
	return []ProviderMethod{
		{
			Path: "/api/concurrency-sls/qps",
			Func: func(w http.ResponseWriter, req *http.Request) {
				renderResponse(w, []byte(fmt.Sprintf("{\"code\":200, \"sls_qps\": \"%d\", \"total\": \"%d\"}", scf.qps, atomic.LoadInt64(&scf.count))), "application/json")
			},
		},
	}
}

func NewKafkaSinglePartitionFetcher(brokers []string, topic string, offset int64) (*KafkaSinglePartitionFetcher, error) {
	if len(brokers) == 0 {
		Logger.Error("bad broker")
		return nil, errors.New("empty broker")
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		Partition:      0,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
	})
	r.SetOffset(offset)
	Logger.Info("created KafkaSinglePartitionFetcher", zap.Strings("brokers", brokers), zap.String("topic", topic), zap.Int64("offset", offset))
	return &KafkaSinglePartitionFetcher{
		baseFetcher: newBaseFetcher(),
		reader:      r,
	}, nil
}

func (kspf *KafkaSinglePartitionFetcher) Stop() {
	kspf.baseFetcher.Stop()
	if kspf.parent != nil {
		kspf.parent.Notify(kspf, StatusStopped)
	}
}

func (kspf *KafkaSinglePartitionFetcher) Finish() {
	kspf.baseFetcher.Finish()
	if kspf.parent != nil {
		kspf.parent.Notify(kspf, StatusFinished)
	}
}

func (kspf *KafkaSinglePartitionFetcher) Start() error {
	kspf.baseFetcher.start()
	if kspf.parent != nil {
		kspf.parent.Notify(kspf, StatusRunning)
	}

	go func() {
		var last int64
		var current int64
		for {
			time.Sleep(time.Second)
			current = atomic.LoadInt64(&kspf.counter)
			kspf.qps = current - last
			last = current
			Logger.Info("QPS of KafkaSinglePartionFetcher", zap.Int64("qps", kspf.qps), zap.Int64("count", last))
		}
	}()

	for {
		msg, err := kspf.reader.ReadMessage(context.Background())
		atomic.AddInt64(&kspf.counter, 1)
		if err != nil {
			Logger.Error("occur error when reading message", zap.Error(err))
			return err
		}

		if kspf.analyzer == nil {
			continue
		}

		log := kspf.analyzer.Analyze(msg.Value)
		if log == nil {
			continue
		}

		if log.OccurAt.After(kspf.end) {
			Logger.Info("time of log record is later than end time, finish fetching from kafka", zap.Time("occurAt", log.OccurAt), zap.Time("end", kspf.end))
			break
		}

		if !log.OccurAt.Before(kspf.begin) {
			kspf.output <- log
		}
	}

	return nil
}

func (kspf *KafkaSinglePartitionFetcher) Provide() []ProviderMethod {
	return []ProviderMethod{
		{
			Path: "/api/kafka/qps",
			Func: func(w http.ResponseWriter, req *http.Request) {
				renderResponse(w, []byte(fmt.Sprintf("{\"code\":200, \"kafka_qps\": \"%d\", \"total\": \"%d\"}", kspf.qps, atomic.LoadInt64(&kspf.counter))), "application/json")
			},
		},
	}
}
