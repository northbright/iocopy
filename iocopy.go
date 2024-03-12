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

	// DefaultInterval is the default interval to report the number of bytes written.
	DefaultInterval = 500 * time.Millisecond
)

// Event is the interface that wraps String method.
// When Start is called, it'll return a channel for the caller to receive
// IO copy related events.
// Available events:
// (1). EventWritten - n bytes have been written successfully.
// (2). EventError - an error occurs and the goroutine exits.
// (3). EventStop - IO copy stopped.
// (4). EventOK - IO copy succeeded.
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

// Copied returns the number of bytes copied previously.
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

// Err returns the the context error that explains why IO copying is stopped.
// It can be context.Canceled or context.DeadlineExceeded.
func (e *EventStop) Err() error {
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
	dst io.Writer,
	src io.Reader,
	bufSize uint,
	interval time.Duration,
	isTotalKnown bool,
	total uint64,
	prevCopied uint64,
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
// isTotalKnown: if total bytes to copy is known or not.
// total: total number of bytes to copy.
// prevCopied: the number of bytes prevCopied previously.
// The number of bytes to copy for this time = total - prevCopied.
//
// It returns a channel to receive IO copy events.
// Available events:
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
	bufSize uint,
	interval time.Duration,
	isTotalKnown bool,
	total uint64,
	prevCopied uint64) <-chan Event {
	ch := make(chan Event)

	go cp(ctx, dst, src, bufSize, interval, true, total, prevCopied, ch)

	return ch
}

/*
// CopyFile copies src to dst and returns the number of bytes prevCopied.
// onProgress will be called when progress is updated.
// It blocks the caller's goroutine until the copy is done or stopped.
func CopyFile(
	ctx context.Context,
	dst, src string,
	bufSize uint,
	onProgress OnProgress) (uint64, error) {

	ch := make(chan Event)

	// Open dst file.
	dstFile, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer dstFile.Close()

	// Open src file.
	srcFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcFile.Close()

	// Get the size of src file.
	fi, err := srcFile.Stat()
	if err != nil {
		return 0, err
	}

	// Check if src's a regular file.
	if !fi.Mode().IsRegular() {
		return 0, fmt.Errorf("not regular file")
	}

	// Get total size of src.
	total := uint64(fi.Size())

	go cp(ctx, dstFile, srcFile, bufSize, DefaultInterval, true, total, 0, ch)

	return processEvent(false, total, onProgress, ch)
}

// copyFileFS copies src in a fs.FS to dst and returns the number of bytes prevCopied.
// onProgress will be called when progress is updated.
// It blocks the caller's goroutine until the copy is done or stopped.
func CopyFileFS(
	ctx context.Context,
	dst string,
	srcFS fs.FS,
	src string,
	bufSize uint,
	onProgress OnProgress) (uint64, error) {

	ch := make(chan Event)

	// Open dst file.
	dstFile, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer dstFile.Close()

	// Open src file.
	srcFile, err := srcFS.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcFile.Close()

	// Get the size of src file.
	fi, err := srcFile.Stat()
	if err != nil {
		return 0, err
	}

	// Check if src's a regular file.
	if !fi.Mode().IsRegular() {
		return 0, fmt.Errorf("not regular file")
	}

	// Get total size of src.
	total := uint64(fi.Size())

	go cp(ctx, dstFile, srcFile, bufSize, DefaultInterval, true, total, 0, ch)

	return processEvent(false, total, onProgress, ch)
}
*/
