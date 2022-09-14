package kafka

import (
	"context"
	"time"
)

type (
	Reader interface {
		//ApplyConfig(op)
		SetOffset(offset int64) error
		ReadMessage(ctx context.Context) (Message, error)
		Close()
	}

	Message struct {
		Topic     string
		Partition int
		Offset    int64
		Key       []byte
		Value     []byte
		Headers   []Header
		Time      time.Time
	}

	Header struct {
		Key   []byte
		Value []byte
	}
)
