package iocopy

import (
	"context"
	"fmt"
	"io"
	"time"
)

const (
	// DefaultBufSize is the default buffer size.
	DefaultBufSize = int64(32 * 1024)

	// DefaultInterval is the default interval to report count of written bytes.
	DefaultInterval = 500 * time.Millisecond
)

type Event interface {
	// stringer
	String() string
}

type EventWritten struct {
	written int64
}

func newEventWritten(written int64) *EventWritten {
	return &EventWritten{written: written}
}

func (e *EventWritten) String() string {
	return fmt.Sprintf("%d bytes written", e.written)
}

func (e *EventWritten) Written() int64 {
	return e.written
}

type EventError struct {
	err error
}

func newEventError(err error) *EventError {
	return &EventError{err: err}
}

func (e *EventError) String() string {
	return e.err.Error()
}

func (e *EventError) Err() error {
	return e.err
}

type EventOK struct {
	written int64
}

func newEventOK(written int64) *EventOK {
	return &EventOK{written: written}
}

func (e *EventOK) String() string {
	return fmt.Sprintf("written OK(total: %d bytes)", e.written)
}

func (e *EventOK) Written() int64 {
	return e.written
}

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
			if ticker != nil {
				ticker.Stop()
			}
			close(ch)
		}()

		if bufSize <= 0 {
			bufSize = DefaultBufSize
		}
		buf := make([]byte, bufSize)

		if interval <= 0 {
			interval = DefaultInterval
		}
		ticker = time.NewTicker(interval)

		for {
			select {
			case <-ticker.C:
				if written != oldWritten {
					oldWritten = written
					ch <- newEventWritten(written)
				}

			case <-ctx.Done():
				ch <- newEventError(ctx.Err())
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

					ch <- newEventOK(written)
					return
				} else {
					if n, err = dst.Write(buf[:n]); err != nil {
						ch <- newEventError(err)
						return
					}
				}

				written += int64(n)
			}
		}

	}(ch)

	return ch
}
