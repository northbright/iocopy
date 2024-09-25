package progress

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/northbright/iocopy"
)

var (
	DefaultInterval = time.Millisecond * 500
)

// Percent returns the percentage.
// total: total number of the bytes to copy.
// prev: the number of the bytes copied previously.
// current: the number of bytes written currently.
func Percent(total, prev, current int64) float32 {
	if total == 0 {
		return 100
	}

	if total < 0 {
		return 0
	}

	if prev+current < 0 {
		return 0
	}

	return float32(float64(prev+current) / (float64(total) / float64(100)))
}

type Progress struct {
	total    int64
	prev     int64
	current  int64
	old      int64
	lock     sync.RWMutex
	fn       iocopy.OnWrittenFunc
	interval time.Duration
}

type Option func(p *Progress)

func OnWritten(fn iocopy.OnWrittenFunc) Option {
	return func(p *Progress) {
		p.fn = fn
	}
}

func Interval(d time.Duration) Option {
	return func(p *Progress) {
		p.interval = d
	}
}

func New(total, prev int64, options ...Option) *Progress {
	p := &Progress{
		total: total,
		prev:  prev,
	}

	for _, option := range options {
		option(p)
	}

	if p.interval <= 0 {
		p.interval = DefaultInterval
	}

	return p
}

func (p *Progress) Write(b []byte) (n int, err error) {
	n = len(b)
	p.lock.Lock()
	p.current += int64(n)
	p.lock.Unlock()
	return n, nil
}

func (p *Progress) callback() {
	if p.fn != nil {
		p.lock.RLock()
		if p.current != p.old {
			p.fn(p.total, p.prev, p.current, Percent(p.total, p.prev, p.current))
			p.old = p.current
		}
		p.lock.RUnlock()
	}
}

func (p *Progress) Start(ctx context.Context, chExit <-chan struct{}) {
	ch := time.Tick(p.interval)

	go func() {
		for {
			select {
			case <-chExit:
				log.Printf("on exit")
				p.callback()
				return
			case <-ctx.Done():
				log.Printf("on ctx.Done()")
				return
			case <-ch:
				p.callback()
			}
		}
	}()
}
