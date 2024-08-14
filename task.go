package iocopy

import (
	"context"
	"io"
	"time"
)

// OnWritten is the type of function called by [Do] when n bytes is written(copied) successfully.
type OnWritten func(isTotalKnown bool, total, copied, written uint64, percent float32)

// OnStop is the type of function called by [Do] when copy is stopped. The cause parameter is returned by context.Err().
type OnStop func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error)

// OnOK is the type of function called by [Do] when copy is done.
type OnOK func(isTotalKnown bool, total, copied, written uint64, percent float32)

// OnError is the type of function called by [Do] when error occurs.
type OnError func(err error)

// Task is the interface of io copy task which is passed to [Do].
type Task interface {
	Writer() (io.Writer, error)
	Reader() (io.Reader, error)
	Total() (bool, uint64)
	Copied() uint64
	SetCopied(uint64)
}

// Do does io copy task and block caller's go routine until an error occurs or copy stopped by user or copy is done.
// Parameters:
// ctx: context.Context.
// It can be created using context.WithCancel, context.WithDeadline,
// context.WithTimeout...
// t: [Task]
// bufSize: size of the buffer. It'll create a buffer in the new goroutine according to the buffer size.
// interval: Interval to reports n bytes written(copied) during the IO copy.
func Do(
	ctx context.Context,
	t Task,
	bufSize uint,
	interval time.Duration,
	onWritten OnWritten,
	onStop OnStop,
	onOK OnOK,
	onError OnError) {
	isTotalKnown, total := t.Total()
	copied := t.Copied()

	// Get io.Writer.
	w, err := t.Writer()
	if err != nil {
		if onError != nil {
			onError(err)
		}
	}

	wc, ok := w.(io.WriteCloser)
	if ok {
		defer wc.Close()
	}

	// Get io.Reader.
	r, err := t.Reader()
	if err != nil {
		if onError != nil {
			onError(err)
		}
	}

	rc, ok := r.(io.ReadCloser)
	if ok {
		defer rc.Close()
	}

	ch := Start(
		ctx,
		w,
		r,
		bufSize,
		interval,
		isTotalKnown,
		total,
		copied,
	)

	// Read the events from the channel.
	for event := range ch {
		switch ev := event.(type) {
		case *EventWritten:
			if onWritten != nil {
				onWritten(
					ev.IsTotalKnown(),
					ev.Total(),
					ev.Copied(),
					ev.Written(),
					ev.Percent(),
				)
			}

		case *EventStop:
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

		case *EventOK:
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

		case *EventError:
			err := ev.Err()

			if onError != nil {
				onError(err)
			}
		}
	}
}
