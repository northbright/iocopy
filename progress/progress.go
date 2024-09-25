package progress

import (
	"context"
	"log"
	"sync"
	"time"
)

var (
	// DefaultInterval is the default interval of the tick of callback to report progress.
	DefaultInterval = time.Millisecond * 500
)

// OnWrittenFunc is the callback function when bytes are copied successfully.
// total: total number of bytes to copy.
// prev: number of bytes copied previously.
// current: number of bytes copied in current copy.
// percent: percent copied.
type OnWrittenFunc func(total, prev, current int64, percent float32)

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

// Progress implements the [io.Writer] interface.
// Call [*Progress.Start] to starts a new goroutine to report progress.
type Progress struct {
	total    int64
	prev     int64
	current  int64
	old      int64
	lock     sync.RWMutex
	fn       OnWrittenFunc
	interval time.Duration
}

// Option represents the optional parameter when new a [Progress].
type Option func(p *Progress)

// Prev returns an option to set the number of copied bytes previously.
func Prev(prev int64) Option {
	return func(p *Progress) {
		p.prev = prev
	}
}

// OnWritten returns an option to set the callback function to report progress.
func OnWritten(fn OnWrittenFunc) Option {
	return func(p *Progress) {
		p.fn = fn
	}
}

// Interval returns an option to set the tick interval for the callback function.
func Interval(d time.Duration) Option {
	return func(p *Progress) {
		p.interval = d
	}
}

// New creates a [Progress].
// total: total number of bytes to copy.
// options: optional parameters returned by [Prev], [OnWritten] and [Interval].
func New(total int64, options ...Option) *Progress {
	p := &Progress{
		total: total,
	}

	for _, option := range options {
		option(p)
	}

	if p.interval <= 0 {
		p.interval = DefaultInterval
	}

	return p
}

// Write implements [io.Writer] interface.
func (p *Progress) Write(b []byte) (n int, err error) {
	n = len(b)
	p.lock.Lock()
	p.current += int64(n)
	p.lock.Unlock()
	return n, nil
}

// callback calls the callback function to report progress.
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

// Start starts a new goroutine and tick to call the callback to report progress.
// It exits when it receives data from ctx.Done() or chExit.
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
