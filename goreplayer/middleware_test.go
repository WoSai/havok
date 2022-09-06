package replayer

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestTimer_Duration(t *testing.T) {
	ti := &timer{}
	ti.Start()
	time.Sleep(10 * time.Millisecond)
	ti.End()
	assert.Greater(t, ti.Duration(), 10*time.Millisecond)
}

func TestTimer_End(t *testing.T) {
	ti := &timer{}
	ti.End()
	assert.Equal(t, ti.Duration(), time.Duration(0))
}
