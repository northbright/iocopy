package iocopy

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"time"
)

const (
	// DefaultBufSize is the default buffer size.
	DefaultBufSize = uint(32 * 1024)

	// DefaultInterval is the default interval to report the number of bytes copied.
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
// (5). EventProgress - IO copy progress updated.
type Event interface {
	// stringer
	String() string
}

// EventWritten is the event that n bytes have been written successfully.
type EventWritten struct {
	written uint64
}

func newEventWritten(written uint64) *EventWritten {
	return &EventWritten{written: written}
}

// String implements the stringer interface.
func (e *EventWritten) String() string {
	return fmt.Sprintf("%d bytes written", e.written)
}

// Written returns the number of bytes written successfuly.
func (e *EventWritten) Written() uint64 {
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
	err error
	ew  *EventWritten
}

func newEventStop(err error, written uint64) *EventStop {
	return &EventStop{
		err: err,
		ew:  newEventWritten(written),
	}
}

// String implements the stringer interface.
func (e *EventStop) String() string {
	return fmt.Sprintf("written stopped(reason: %v, written bytes: %d)", e.err, e.ew.Written())
}

// Written returns the number of bytes written successfuly.
func (e *EventStop) Written() uint64 {
	return e.ew.Written()
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

func newEventOK(written uint64) *EventOK {
	return &EventOK{ew: newEventWritten(written)}
}

// String implements the stringer interface.
func (e *EventOK) String() string {
	return fmt.Sprintf("copy OK(written bytes: %d", e.ew.Written())
}

// Written returns the number of bytes written successfuly.
func (e *EventOK) Written() uint64 {
	return e.ew.Written()
}

// EventProgress is the event that IO copy progress updated.
type EventProgress struct {
	ew      *EventWritten
	percent float32
}

func newEventProgress(
	written uint64,
	percent float32) *EventProgress {
	return &EventProgress{
		ew:      newEventWritten(written),
		percent: percent,
	}
}

// String implements the stringer interface.
func (e *EventProgress) String() string {
	return fmt.Sprintf("progress updated: %.2f%%", e.percent)
}

// Written returns the number of bytes written successfuly.
func (e *EventProgress) Written() uint64 {
	return e.ew.Written()
}

// EventWritten returns the percentage of the copy progress.
// percent = number of bytes copied + bytes written currently / number of bytes copied + number of bytes to copy.
func (e *EventProgress) Percent() float32 {
	return e.percent
}

// ComputePercent returns the current and total percentage.
// It assumes that nBytesToCopy + nBytesCopied = total number of bytes of the source.
func ComputePercent(nBytesToCopy, nBytesCopied, written uint64, done bool) float32 {
	total := nBytesToCopy + nBytesCopied

	if done {
		// Return 100 percent when copy is done,
		// even if total is 0.
		return 100
	}

	if total == 0 {
		return 0
	}

	return float32(float64(nBytesCopied+written) / (float64(total) / float64(100)))
}

// updateProgress returns the new current and total percent, send an EventProgress event to the channel if the progress was updated.
func updateProgress(withProgress bool, nBytesToCopy, nBytesCopied, written uint64, done bool, oldPercent *float32, ch chan<- Event) {
	if !withProgress {
		return
	}
	percent := ComputePercent(
		nBytesToCopy,
		nBytesCopied,
		written,
		done)

	if percent != *oldPercent {
		*oldPercent = percent
		ch <- newEventProgress(written, percent)
	}
}

func cp(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	bufSize uint,
	interval time.Duration,
	withProgress bool,
	nBytesToCopy uint64,
	nBytesCopied uint64,
	ch chan Event) {
	var (
		written    uint64       = 0
		oldWritten uint64       = 0
		oldPercent float32      = 0
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
				ch <- newEventWritten(written)

				// Update the progress.
				updateProgress(
					withProgress,
					nBytesToCopy,
					nBytesCopied,
					written,
					false,
					&oldPercent,
					ch)
			}

		case <-ctx.Done():
			// Context is canceled or
			// context's deadline exceeded.

			// Stop the ticker.
			if ticker != nil {
				ticker.Stop()
			}

			// Update the progress.
			updateProgress(
				withProgress,
				nBytesToCopy,
				nBytesCopied,
				written,
				false,
				&oldPercent,
				ch)

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

				// Update the progress.
				updateProgress(
					withProgress,
					nBytesToCopy,
					nBytesCopied,
					written,
					true,
					&oldPercent,
					ch)

				// Send an EventOK.
				ch <- newEventOK(written)
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
	interval time.Duration) <-chan Event {
	ch := make(chan Event)

	go cp(ctx, dst, src, bufSize, interval, false, 0, 0, ch)

	return ch
}

// StartWithProgress returns a channel for the caller to receive IO copy events and start a goroutine to do IO copy.
// It's an wrapper of Start and most of the parameters are the same.
// To resume an IO copy:
// (1). Users should save the state and the number of bytes copied after the IO copy stopped(e.g. timeout or user canceled).
// (2). Call StartWithProgress again, make dst and src load the saved state or set the offset according to number of the bytes to copied.
//
// A new event: EventProgress will be sent to the channel when progress was updated.
// Call CurrentPercent and TotalPercent on the event to get the percentages.
// Current percentage: percentage of the current running IO copy.
// Total percentage: percentage of the whole IO copy which may be separated into multiple sub IO copies due to users' stopping / resuming the IO copy.
// See Start for more information.
func StartWithProgress(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	bufSize uint,
	interval time.Duration,
	nBytesToCopy uint64,
	nBytesCopied uint64) <-chan Event {
	ch := make(chan Event)

	go cp(ctx, dst, src, bufSize, interval, true, nBytesToCopy, nBytesCopied, ch)

	return ch
}

// OnProgress presents the callback function on the copy progress is updated.
// total and percent are ignored when isTotalSizeUnknown is true.
type OnProgress func(isTotalSizeUnknown bool, total, copied uint64, percent float32)

// processEvent reads events from the channel and processes the events.
func processEvent(isTotalSizeUnknown bool, total uint64, onProgress OnProgress, ch <-chan Event) (uint64, error) {
	var (
		err     error
		written uint64
	)

	for event := range ch {
		switch ev := event.(type) {
		case *EventWritten:
			// n bytes have been written successfully.

		case *EventProgress:
			written = ev.Written()
			if onProgress != nil {
				if isTotalSizeUnknown {
					// total size is unknown.
					onProgress(isTotalSizeUnknown, 0, written, 0)
				} else {
					onProgress(isTotalSizeUnknown, total, written, ev.Percent())
				}
			}
		case *EventStop:
			// Context is canceled or
			// context's deadline exceeded.
			written = ev.Written()
			// Save the error.
			err = ev.Err()

		case *EventError:
			// an error occured.
			err = ev.Err()

		case *EventOK:
			// IO copy succeeded.
			written = ev.Written()
		}
	}
	return written, err
}

// CopyFile copies src to dst and returns the number of bytes copied.
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

// copyFileFS copies src in a fs.FS to dst and returns the number of bytes copied.
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
