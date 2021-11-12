package types

import (
	"sync"
)

type (
	Session struct {
		store sync.Map
	}

	SessionPool struct {
		pool *sync.Pool
	}
)

func NewSession() *Session {
	return &Session{}
}

func (s *Session) Get(key string) (interface{}, bool) {
	return s.store.Load(key)
}

func (s *Session) Put(key string, val interface{}) {
	s.store.Store(key, val)
}

func (s *Session) Reset() {
	s.store.Range(func(key, value interface{}) bool {
		s.store.Delete(key)
		return true
	})
}

func (s *Session) Copy(session *Session) {
	session.store.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			s.store.Store(k, value)
		}
		return true
	})
}

func NewSessionPool() *SessionPool {
	return &SessionPool{
		pool: &sync.Pool{New: func() interface{} { return NewSession() }},
	}
}

func (sp *SessionPool) Get() *Session {
	return sp.pool.Get().(*Session)
}

func (sp *SessionPool) Put(s *Session) {
	s.Reset()
	sp.pool.Put(s)
}
