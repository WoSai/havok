package dispatcher

type (
	// Analyzer 日志内容解析器
	Analyzer interface {
		Use(...LogParser)
		Analyze([]byte) *LogRecordWrapper
	}

	LogParserPool interface {
		Register(... LogParser)
		Take(name string) LogParser
	}

	// BaseAnalyzer Analyzer的简单实现
	BaseAnalyzer struct {
		analyzers []LogParser
		LogParserPool
	}

	PluginLogParserPool struct {
		cacheLogParser map[string]LogParser
	}

	// AnalyzeFunc 日志内容解析方法
	LogParser interface {
		Name() string
		Parse([]byte) (*LogRecordWrapper, bool)
	}
)

// NewBaseAnalyzer BaseAnalyzer实例化
func NewBaseAnalyzer(pool LogParserPool, enables []string) (*BaseAnalyzer) {
	base := &BaseAnalyzer{
		analyzers:     []LogParser{},
		LogParserPool: pool,
	}
	for _, enable := range enables {
		a := base.Take(enable)
		if a != nil {
			base.Use(a)
		} else {
			Logger.Panic("loss analyzer.handler "+enable)
			return nil
		}
	}
	return base
}

func NewPluginAnalyzerPool(plugins ...string) (PluginLogParserPool, error) {
	var pool = PluginLogParserPool{cacheLogParser: make(map[string]LogParser)}
	for _, path := range plugins {
		an, err := Load(path)
		if err != nil {
			return PluginLogParserPool{}, err
		}
		pool.Register(an)
	}
	return pool, nil
}

func (pool PluginLogParserPool) Register(as ...LogParser) {
	for _, a := range as {
		pool.cacheLogParser[a.Name()] = a
	}
}

func (pool PluginLogParserPool) Take(name string) LogParser {
	if a, ok := pool.cacheLogParser[name]; ok {
		return a
	}
	return nil
}

// Use 添加解析函数
func (bs *BaseAnalyzer) Use(as ...LogParser) {
	if as != nil {
		bs.analyzers = append(bs.analyzers, as...)
	}
}

// Analyze 日志解析行为，返回nil表示日志不匹配
func (bs *BaseAnalyzer) Analyze(data []byte) *LogRecordWrapper {
	log := new(LogRecordWrapper)
	matched := false
	for _, f := range bs.analyzers {
		log, matched = f.Parse(data)
		if matched && !log.OccurAt.IsZero() {
			if log.Header != nil {
				h := copyHeader(log.Header)
				log.Header = h
			}
			return log
		}
	}
	return nil
}

func copyHeader(data map[string]string) map[string]string {
	m := make(map[string]string)
	for k, v := range data {
		m[k] = v
	}
	return m
}
