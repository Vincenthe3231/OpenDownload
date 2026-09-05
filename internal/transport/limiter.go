package transport

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
)

// ConnectionLimiter caps simultaneous HTTP response bodies across clients.
// A nil limiter leaves a client unconstrained.
type ConnectionLimiter struct {
	slots  chan struct{}
	active atomic.Int64
}

// NewConnectionLimiter returns a limiter for limit simultaneous response
// bodies. A nonpositive limit disables limiting and returns nil.
func NewConnectionLimiter(limit int) *ConnectionLimiter {
	if limit <= 0 {
		return nil
	}
	return &ConnectionLimiter{slots: make(chan struct{}, limit)}
}

func (limiter *ConnectionLimiter) acquire(ctx context.Context) error {
	if limiter == nil {
		return nil
	}
	select {
	case limiter.slots <- struct{}{}:
		limiter.active.Add(1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (limiter *ConnectionLimiter) release() {
	if limiter == nil {
		return
	}
	<-limiter.slots
	limiter.active.Add(-1)
}

// Active returns the number of currently open, limited response bodies.
func (limiter *ConnectionLimiter) Active() int64 {
	if limiter == nil {
		return 0
	}
	return limiter.active.Load()
}

type limitedReadCloser struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (body *limitedReadCloser) Close() error {
	err := body.ReadCloser.Close()
	body.once.Do(body.release)
	return err
}
