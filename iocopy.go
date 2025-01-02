package iocopy

import (
	"context"
	"io"
	"log"
	"runtime"
	"sync"
	"time"
)

var (
	// DefaultInterval is the default interval of the tick of callback to report progress.
	DefaultInterval = time.Millisecond * 500
)

// readFunc is used to implement [io.Reader] interface and capture the [context.Context] parameter.
type readFunc func(p []byte) (n int, err error)

// Read implements [io.Reader] interface.
func (rf readFunc) Read(p []byte) (n int, err error) {
	return rf(p)
}

// writeFunc is used to implement [io.Writer] interface and capture the [context.Context] parameter.
type writeFunc func(p []byte) (n int, err error)

// Write implements [io.Writer] interface.
func (wf readFunc) Write(p []byte) (n int, err error) {
	return wf(p)
}

// OnWrittenFunc is the callback function when bytes are copied successfully.
// total: total number of bytes to copy.
// A negative value indicates total size is unknown and percent should be ignored(always 0).
// prev: number of bytes copied previously.
// current: number of bytes copied in current copy.
// percent: percent copied.
type OnWrittenFunc func(total, prev, current int64, percent float32)

// Percent returns the percentage.
// total: total number of the bytes to copy.
// A negative value indicates total size is unknown and it returns 0 as percent.
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
type Progress struct {
	total    int64
	prev     int64
	current  int64
	old      int64
	done     bool
	lock     sync.RWMutex
	fn       OnWrittenFunc
	interval time.Duration
}

// ProgressOption represents the optional parameter when new a [Progress].
type ProgressOption func(p *Progress)

// Prev returns an option to set the number of written bytes previously.
// It's used to calculate the percent when resume an IO copy.
// It sets prev to 0 by default if it's not provided.
func Prev(prev int64) ProgressOption {
	return func(p *Progress) {
		p.prev = prev
	}
}

// Interval returns an option to set the tick interval for the callback function.
// If no interval option specified, it'll use [DefaultInterval].
func Interval(d time.Duration) ProgressOption {
	return func(p *Progress) {
		p.interval = d
	}
}

// NewProgress creates a [Progress].
// total: total number of bytes to copy.
// A negative value indicates total size is unknown and it always reports 0 as the percent in the OnWrittenFunc.
// prev: number of bytes copied previously.
// options: optional parameters returned by [Prev] and [Interval].
func NewProgress(total int64, fn OnWrittenFunc, options ...ProgressOption) *Progress {
	p := &Progress{
		total: total,
		fn:    fn,
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

	if p.total == p.prev+p.current {
		p.callback()
		p.done = true
	}
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
// It exits when it receives data from ctx.Done().
func (p *Progress) Start(ctx context.Context) {
	if p.fn == nil {
		return
	}

	ch := time.Tick(p.interval)

	go func() {
		defer func() {
			log.Printf("progress goroutine exited")
		}()

		for {
			select {
			case <-ctx.Done():
				p.callback()
				return
			case <-ch:
				p.callback()
			default:
				if p.done == true {
					return
				}
				runtime.Gosched()
			}
		}
	}()
}

// CopyWithProgress wraps [io.Copy]. It accepts [context.Context] to make IO copy cancalable.
// It also accepts callback function on bytes written to report progress.
func CopyWithProgress(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	total int64,
	fn OnWriteFunc,
	options ...ProgressOption) (written int64, err error) {
	//progress := new(

	return io.Copy(
		writeFunc(func(p []byte) (n int, err error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Error()
			default:
				return dst.Write(p)
			}
		}),
		readFunc(func(p []byte) (n int, err error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default:
				return src.Read(p)
			}
		}),
	)
}

// Copy wraps [io.Copy] and accepts [context.Context] parameter.
func Copy(ctx context.Context, dst io.Writer, src io.Reader, options ...ProgressProgressOption) (written int64, err error) {
	//progress := new(

	return io.Copy(
		writeFunc(func(p []byte) (n int, err error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Error()
			default:
				return dst.Write(p)
			}
		}),
		readFunc(func(p []byte) (n int, err error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default:
				return src.Read(p)
			}
		}),
	)
}

// CopyBuffer wraps [io.CopyBuffer] and accepts [context.Context] parameter.
func CopyBuffer(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	return io.CopyBuffer(
		dst,
		readFunc(func(p []byte) (n int, err error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default:
				return src.Read(p)
			}
		}),
		buf,
	)
}
