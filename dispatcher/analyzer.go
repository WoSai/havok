package dispatcher

type (
	// Analyzer 日志内容解析器
	Analyzer interface {
		Use(...AnalyzeFunc)
		Analyze([]byte) *LogRecordWrapper
	}

	// BaseAnalyzer Analyzer的简单实现
	BaseAnalyzer struct {
		analyzers []AnalyzeFunc
	}

	// AnalyzeFunc 日志内容解析方法
	AnalyzeFunc func([]byte) (*LogRecordWrapper, bool)
)

// NewBaseAnalyzer BaseAnalyzer实例化
func NewBaseAnalyzer() *BaseAnalyzer {
	return &BaseAnalyzer{
		analyzers: []AnalyzeFunc{},
	}
}

// Use 添加解析函数
func (bs *BaseAnalyzer) Use(as ...AnalyzeFunc) {
	if as != nil {
		bs.analyzers = append(bs.analyzers, as...)
	}
}

// Analyze 日志解析行为，返回nil表示日志不匹配
func (bs *BaseAnalyzer) Analyze(data []byte) *LogRecordWrapper {
	log := new(LogRecordWrapper)
	matched := false
	for _, f := range bs.analyzers {
		log, matched = f(data)
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
