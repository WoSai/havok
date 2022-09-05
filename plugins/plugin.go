package main

import (
	"github.com/wosai/havok/pkg"
	"time"
)

type (
	fileFetcher struct {
		ch chan<- *pkg.LogRecordWrapper
	}
)

func newFileFetcher() *fileFetcher {
	return &fileFetcher{}
}

func (f *fileFetcher) Read(ch chan<- *pkg.LogRecordWrapper) {
	f.start(ch)
}

func (f *fileFetcher) start(ch chan<- *pkg.LogRecordWrapper) {
	f.ch = ch
	go func() {
		for {
			f.ch <- &pkg.LogRecordWrapper{
				OccurAt: time.Now(),
			}
			time.Sleep(time.Second)
		}
	}()
}

func InitPlugin(opt *pkg.PluginOption) error {
	return nil
}

func RegisterPlugin(m pkg.Manager) error {
	m.RegisterFetcher(newFileFetcher())
	return nil
}
