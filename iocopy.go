package iocopy

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"time"
)

const (
	// DefaultBufSize is the default buffer size.
	DefaultBufSize = uint(32 * 1024)

	// DefaultRefreshRate is the default refresh rate of EventWritten / OnWritten callback.
	DefaultRefreshRate = 500 * time.Millisecond
)

// Event is the interface that wraps String method.
// When Start is called, it'll return a channel for the caller to receive
// IO copy related events.
// Available events:
// (1). EventWritten - n bytes have been written successfully.
// (2). EventStop - IO copy stopped.
// (3). EventOK - IO copy succeeded.
// (4). EventError - an error occurs and the goroutine exits.
type Event interface {
	// stringer
	String() string
}

// EventWritten is the event that n bytes have been written successfully.
type EventWritten struct {
	isTotalKnown bool
	total        uint64
	prevCopied   uint64
	written      uint64
	// if copy is done
	done bool
}

func newEventWritten(isTotalKnown bool, total, prevCopied, written uint64, done bool) *EventWritten {
	return &EventWritten{
		isTotalKnown: isTotalKnown,
		total:        total,
		prevCopied:   prevCopied,
		written:      written,
		done:         done,
	}
}

// String implements the stringer interface.
func (e *EventWritten) String() string {
	return fmt.Sprintf("%d bytes written", e.written)
}

// IsTotalKnown returns if total bytes to copy is known or not.
func (e *EventWritten) IsTotalKnown() bool {
	return e.isTotalKnown
}

// Total returns the number of bytes to copy.
// It's ignored if total bytes to copy is unknown.
func (e *EventWritten) Total() uint64 {
	return e.total
}

// PrevCopied returns the number of bytes copied previously.
func (e *EventWritten) PrevCopied() uint64 {
	return e.prevCopied
}

// Copied returns the number of bytes copied.
// It's equals the number of bytes copied prevously + the number of bytes written.
func (e *EventWritten) Copied() uint64 {
	return e.prevCopied + e.written
}

// Written returns the number of bytes written successfuly.
func (e *EventWritten) Written() uint64 {
	return e.written
}

// Done returns if copy is done.
// It returns false when copy is stopped.
func (e *EventWritten) Done() bool {
	return e.done
}

// Percent returns the percentage of copy progress.
func (e *EventWritten) Percent() float32 {
	return ComputePercent(e.total, e.prevCopied, e.written, e.done)
}

// EventOK is the event that IO copy stopped.
type EventStop struct {
	err error
	ew  *EventWritten
}

func newEventStop(err error, isTotalKnown bool, total, prevCopied, written uint64) *EventStop {
	return &EventStop{
		err: err,
		ew:  newEventWritten(isTotalKnown, total, prevCopied, written, false),
	}
}

// String implements the stringer interface.
func (e *EventStop) String() string {
	return fmt.Sprintf("written stopped(reason: %v, written bytes: %d)", e.err, e.ew.Written())
}

// EventWritten returns the associated EventWritten event.
func (e *EventStop) EventWritten() *EventWritten {
	return e.ew
}

// Cause returns a non-nil error explaining why IO copying is canceled(stopped).
func (e *EventStop) Cause() error {
	return e.err
}

// EventOK is the event that IO copy succeeded.
type EventOK struct {
	ew *EventWritten
}

func newEventOK(isTotalKnown bool, total, prevCopied, written uint64) *EventOK {
	return &EventOK{ew: newEventWritten(isTotalKnown, total, prevCopied, written, true)}
}

// String implements the stringer interface.
func (e *EventOK) String() string {
	return fmt.Sprintf("copy OK(written bytes: %d", e.ew.Written())
}

// EventWritten returns the associated EventWritten event.
func (e *EventOK) EventWritten() *EventWritten {
	return e.ew
}

// EventError is the event that an error occurs.
type EventError struct {
	err error
}

func newEventError(err error) *EventError {
	return &EventError{err: err}
}

// String implements the stringer interface.
func (e *EventError) String() string {
	return e.err.Error()
}

// Err returns the error occured during IO copy.
func (e *EventError) Err() error {
	return e.err
}

// ComputePercent returns the percentage.
// total: total number of the bytes to copy.
// prevCopied: the number of the bytes copied previously.
// written: the number of bytes written.
// done: if copy is done.
func ComputePercent(total, prevCopied, written uint64, done bool) float32 {
	if done {
		// Return 100 percent when copy is done,
		// even if total is 0.
		return 100
	}

	if total == 0 {
		return 0
	}

	return float32(float64(prevCopied+written) / (float64(total) / float64(100)))
}

func cp(
	ctx context.Context,
	src io.Reader,
	dst io.Writer,
	isTotalKnown bool,
	total uint64,
	prevCopied uint64,
	bufSize uint,
	refreshRate time.Duration,
	ch chan Event) {
	var (
		written, oldWritten uint64
		ticker              *time.Ticker = nil
	)

	defer func() {
		// Stop ticker.
		if ticker != nil {
			ticker.Stop()
		}

		// Close the event channel.
		close(ch)
	}()

	if bufSize == 0 {
		bufSize = DefaultBufSize
	}
	buf := make([]byte, bufSize)

	if refreshRate > 0 {
		ticker = time.NewTicker(refreshRate)
	} else {
		// If refreshRate <= 0, use default refreshRate to create the ticker
		// and stop it immediately.
		ticker = time.NewTicker(DefaultRefreshRate)
		ticker.Stop()
	}

	for {
		select {
		case <-ticker.C:
			if written != oldWritten {
				oldWritten = written
				ch <- newEventWritten(isTotalKnown, total, prevCopied, written, false)
			}

		case <-ctx.Done():
			// Context is canceled or
			// context's deadline exceeded.

			// Stop the ticker.
			if ticker != nil {
				ticker.Stop()
			}

			ch <- newEventStop(ctx.Err(), isTotalKnown, total, prevCopied, written)
			return

		default:
			n, err := src.Read(buf)
			if err != nil && err != io.EOF {
				ch <- newEventError(err)
				return
			}

			// All done.
			if n == 0 {
				// Stop ticker.
				if ticker != nil {
					ticker.Stop()
				}

				// Send an EventOK.
				ch <- newEventOK(isTotalKnown, total, prevCopied, written)
				return
			} else {
				if n, err = dst.Write(buf[:n]); err != nil {
					ch <- newEventError(err)
					return
				}
			}

			written += uint64(n)

			// Let other waiting goroutines to run.
			runtime.Gosched()
		}
	}
}

type Copier struct {
	src          io.Reader
	dst          io.Writer
	isTotalKnown bool
	total        uint64
	prevCopied   uint64
	bufSize      uint
	refreshRate  time.Duration
}

type Option func(c *Copier)

// BufSize returns the option for buffer size.
func BufSize(size uint) Option {
	return func(c *Copier) {
		c.bufSize = size
	}
}

// RefreshRate returns the option for refresh rate of EventWritten / OnWritten callback.
func RefreshRate(refreshRate time.Duration) Option {
	return func(c *Copier) {
		c.refreshRate = refreshRate
	}
}

// New returns a Copier.
// src: io.Reader to copy from.
// dst: io.Writer to copy to.
// isTotalKnown: if total bytes to copy is known or not.
// total: total number of bytes to copy.
// prevCopied: the number of bytes prevCopied previously.
func New(
	src io.Reader,
	dst io.Writer,
	isTotalKnown bool,
	total uint64,
	prevCopied uint64,
	options ...func(c *Copier),
) *Copier {
	c := &Copier{
		src:          src,
		dst:          dst,
		isTotalKnown: isTotalKnown,
		total:        total,
		prevCopied:   prevCopied,
		bufSize:      DefaultBufSize,
		refreshRate:  DefaultRefreshRate,
	}

	for _, option := range options {
		option(c)
	}

	return c
}

// Start returns a channel for the caller to receive IO copy events and start a goroutine to do IO copy.
// ctx: context.Context.
// It can be created using context.WithCancel, context.WithDeadline, context.WithTimeout...
// It returns a channel to receive IO copy events.
// You may use a for-range loop to read events from the channel.
func (c *Copier) Start(ctx context.Context) <-chan Event {
	ch := make(chan Event)

	go cp(ctx, c.src, c.dst, c.isTotalKnown, c.total, c.prevCopied, c.bufSize, c.refreshRate, ch)

	return ch
}

// OnWritten is the type of function called by [Do] called when n bytes is written(copied) successfully.
type OnWritten func(isTotalKnown bool, total, copied, written uint64, percent float32)

// OnStop is the type of function called by [Do] when copy is stopped. The cause parameter is returned by context.Err().
type OnStop func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error)

// OnOK is the type of function called by [Do] when copy is done.
type OnOK func(isTotalKnown bool, total, copied, written uint64, percent float32)

// OnError is the type of function called by [Do] when error occurs.
type OnError func(err error)

/*
// Do does io copy task and block caller's go routine until an error occurs or copy stopped by user or copy is done.
// Parameters:
// ctx: context.Context.
// It can be created using context.WithCancel, context.WithDeadline,
// context.WithTimeout...
// bufSize: size of the buffer. It'll create a buffer in the new goroutine according to the buffer size.
// refreshRate: Interval to reports n bytes written(copied) during the IO copy.
func Do(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	isTotalKnown bool,
	total uint64,
	prevCopied uint64,
	bufSize uint,
	refreshRate time.Duration,
	onWritten OnWritten,
	onStop OnStop,
	onOK OnOK,
	onError OnError) {

	ch := iocopy.Start(
		ctx,
		w,
		r,
		bufSize,
		refreshRate,
		isTotalKnown,
		total,
		copied,
	)

	// Read the events from the channel.
	for event := range ch {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			if onWritten != nil {
				onWritten(
					ev.IsTotalKnown(),
					ev.Total(),
					ev.Copied(),
					ev.Written(),
					ev.Percent(),
				)
			}

		case *iocopy.EventStop:
			ew := ev.EventWritten()

			// Set number of bytes copied for the task.
			t.SetCopied(ew.Copied())

			if onStop != nil {
				onStop(
					ew.IsTotalKnown(),
					ew.Total(),
					ew.Copied(),
					ew.Written(),
					ew.Percent(),
					ev.Cause(),
				)
			}

		case *iocopy.EventOK:
			ew := ev.EventWritten()

			// Set number of bytes copied for the task.
			t.SetCopied(ew.Copied())

			if onOK != nil {
				onOK(
					ew.IsTotalKnown(),
					ew.Total(),
					ew.Copied(),
					ew.Written(),
					ew.Percent(),
				)
			}

		case *iocopy.EventError:
			err := ev.Err()

			if onError != nil {
				onError(err)
			}
		}
	}
}
*/
