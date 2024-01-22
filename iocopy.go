package iocopy

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/northbright/pathelper"
)

const (
	// DefaultBufSize is the default buffer size.
	DefaultBufSize = int64(32 * 1024)

	// DefaultInterval is the default interval to report count of written bytes.
	DefaultInterval = 500 * time.Millisecond
)

// Event is the interface that wraps String method.
// When Start is called, it'll return a channel for the caller to receive
// IO copy related events.
// Currently, there're 4 types of events:
// (1). EventWritten - n bytes have been written successfully.
// (2). EventError - an error occurs and the goroutine exits.
// (3). EventOK - IO copy stopped.
// (4). EventOK - IO copy succeeded.
type Event interface {
	// stringer
	String() string
}

// EventWritten is the event that n bytes have been written successfully.
type EventWritten struct {
	written int64
}

func newEventWritten(written int64) *EventWritten {
	return &EventWritten{written: written}
}

// String implements the stringer interface.
func (e *EventWritten) String() string {
	return fmt.Sprintf("%d bytes written", e.written)
}

// Written returns the count of bytes written successfuly.
func (e *EventWritten) Written() int64 {
	return e.written
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

// EventOK is the event that IO copy stopped.
type EventStop struct {
	err     error
	written int64
}

func newEventStop(err error, written int64) *EventStop {
	return &EventStop{err: err, written: written}
}

// String implements the stringer interface.
func (e *EventStop) String() string {
	return fmt.Sprintf("written stopped(reason: %v, %d bytes written)", e.err, e.written)
}

// Written returns the count of bytes written successfuly.
func (e *EventStop) Written() int64 {
	return e.written
}

// Err returns the the context error that explains why IO copying is stopped.
// It can be context.Canceled or context.DeadlineExceeded.
func (e *EventStop) Err() error {
	return e.err
}

// EventOK is the event that IO copy succeeded.
type EventOK struct {
	written int64
}

func newEventOK(written int64) *EventOK {
	return &EventOK{written: written}
}

// String implements the stringer interface.
func (e *EventOK) String() string {
	return fmt.Sprintf("written OK(total: %d bytes)", e.written)
}

// Written returns the count of bytes written successfuly.
func (e *EventOK) Written() int64 {
	return e.written
}

// Start returns a channel for the caller to receive IO copy events and start a goroutine to do IO copy.
// ctx: context.Context.
// It can be created using context.WithCancel, context.WithDeadline,
// context.WithTimeout...
// dst: io.Writer to copy to.
// src: io.Reader to copy from.
// bufSize: size of the buffer. It'll create a buffer in the new goroutine according to the buffer size.
// interval: It'll create a time.Ticker by given interval to send the EventWritten event to the channel during the IO copy.
// A negative or zero duration causes it to stop the ticker immediately.
// In this case, it'll send the EventWritten to the channel only once when IO copy succeeds.
// You may set it to DefaultInterval.
//
// It returns a channel to receive IO copy events.
// There're 4 types of events:
//
// (1). n bytes have been written successfully.
// It'll send an EventWritten to the channel.
//
// (2). an error occured
// It'll send an EventError to the channel and close the channel.
//
// (3). IO copy stopped(context is canceled or context's deadline exceeded).
// It'll send an EventStop to the channel and close the channel.
//
// (4). IO copy succeeded.
// It'll send an EventOK to the channel and close the channel.
//
// You may use a for-range loop to read events from the channel.
func Start(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	bufSize int64,
	interval time.Duration) <-chan Event {
	ch := make(chan Event)

	go func(ch chan Event) {
		var (
			written    int64        = 0
			oldWritten int64        = 0
			ticker     *time.Ticker = nil
		)

		defer func() {
			// Stop ticker.
			if ticker != nil {
				ticker.Stop()
			}

			// Close the event channel.
			close(ch)
		}()

		if bufSize <= 0 {
			bufSize = DefaultBufSize
		}
		buf := make([]byte, bufSize)

		if interval > 0 {
			ticker = time.NewTicker(interval)
		} else {
			// If interval <= 0, use default interval to create the ticker
			// and stop it immediately.
			ticker = time.NewTicker(DefaultInterval)
			ticker.Stop()
		}

		for {
			select {
			case <-ticker.C:
				if written != oldWritten {
					oldWritten = written
					ch <- newEventWritten(written)
				}

			case <-ctx.Done():
				// Context is canceled or
				// context's deadline exceeded.

				// Stop ticker.
				if ticker != nil {
					ticker.Stop()
				}
				ch <- newEventStop(ctx.Err(), written)
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

					// Send an EventWritten at least before
					// send en EventOK.
					ch <- newEventWritten(written)
					ch <- newEventOK(written)
					return
				} else {
					if n, err = dst.Write(buf[:n]); err != nil {
						ch <- newEventError(err)
						return
					}
				}

				written += int64(n)

				// Let other waiting goroutines to run.
				runtime.Gosched()
			}
		}

	}(ch)

	return ch
}

// CopyFile copies from src to dst and block the caller's goroutine until copy is done.
func CopyFile(ctx context.Context, dst, src string, bufSize int64) (int64, error) {
	// Create dst dir if need.
	dstDir := filepath.Dir(dst)
	if err := pathelper.CreateDirIfNotExists(dstDir, 0755); err != nil {
		return 0, err
	}

	w, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer w.Close()

	r, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	if bufSize < 0 {
		bufSize = DefaultBufSize
	}

	// Start a goroutine to do IO copy.
	ch := Start(
		// Context
		ctx,
		// Writer
		w,
		// Reader(src)
		r,
		// Buffer size
		bufSize,
		// Interval to report written bytes
		0)

	// Read the events from the channel.
	for event := range ch {
		switch ev := event.(type) {
		case *EventStop:
			// Context is canceled or
			// context's deadline exceeded.
			return 0, ev.Err()

		case *EventError:
			// an error occured.
			// Get the error.
			return 0, ev.Err()

		case *EventOK:
			// IO copy succeeded.
			// Get the total count of written bytes.
			n := ev.Written()
			return n, nil
		}
	}

	return 0, fmt.Errorf("unknown error")
}
