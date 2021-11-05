package dispatcher

import (
	"encoding/json"
	"fmt"
	"hash"
	"hash/fnv"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type (
	// HashFNVPool FNV哈希算法
	HashFNVPool struct {
		pool *sync.Pool
	}

	roundtrip struct {
		count uint32
	}
)

var (
	// DefaultFNVHashPool HashFNVPool单例
	DefaultFNVHashPool = NewHashPool()
)

// ParseMSec 将毫秒级别的时间戳转成time.Time对象
func ParseMSec(ms int64) time.Time {
	sec := ms / 1e3
	return time.Unix(sec, (ms-sec*1e3)*1e6)
}

// NewHashPool HashFNVPool的构造函数
func NewHashPool() *HashFNVPool {
	return &HashFNVPool{
		pool: &sync.Pool{New: func() interface{} {
			return fnv.New32a()
		}},
	}
}

// Hash 实现HashFunc
func (hp *HashFNVPool) Hash(s string) uint32 {
	h := hp.pool.Get().(hash.Hash32)
	defer hp.pool.Put(h)
	defer h.Reset()

	h.Write([]byte(s))
	ret := h.Sum32()
	return ret
}

func (rp *roundtrip) hash(s string) uint32 {
	return atomic.AddUint32(&rp.count, 1)
}

func renderJSON(w http.ResponseWriter, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		renderError(w, err)
	} else {
		renderResponse(w, data, "application/json")
	}
}

func renderError(w http.ResponseWriter, err error) {
	renderResponse(
		w,
		[]byte(fmt.Sprintf("{\"code\": 500, \"err_msg\": \"%s\"}", err.Error())),
		"application/json")
}

func renderResponse(w http.ResponseWriter, b []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}
