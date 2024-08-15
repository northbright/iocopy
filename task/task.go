package task

import (
	"context"
	"io"

	"github.com/northbright/iocopy"
)

// Task is the interface of io copy task which is passed to [Do].
type Task interface {
	Writer() io.Writer
	Reader() io.Reader
	Total() (bool, uint64)
	Copied() uint64
	SetCopied(uint64)
}

// Do does IO copy task and block caller's go routine until an error occurs or copy stopped by user or copy is done.
// ctx: context.Context.
// t: [Task]
// onWritten: callback on bytes written.
// onStop: callback on copy is stopped.
// onOK: callback on copy is done.
// onError: callback on an error occurs.
// options: optional parameters(e.g. buffer size, refresh rate...).
func Do(
	ctx context.Context,
	t Task,
	onWritten iocopy.OnWritten,
	onStop iocopy.OnStop,
	onOK iocopy.OnOK,
	onError iocopy.OnError,
	options ...iocopy.Option) {
	isTotalKnown, total := t.Total()
	copied := t.Copied()

	r := t.Reader()
	rc, ok := r.(io.ReadCloser)
	if ok {
		defer rc.Close()
	}

	w := t.Writer()
	wc, ok := w.(io.WriteCloser)
	if ok {
		defer wc.Close()
	}

	c := iocopy.New(
		// Src.
		r,
		// Dst,
		w,
		// Is total bytes to copy known.
		isTotalKnown,
		// Total number of bytes to copy.
		total,
		// The number of bytes copied.
		copied,
		// Options.
		options...,
	)

	c.Do(
		ctx,
		// On bytes written(copied).
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			// Set number of bytes copied for the task.
			t.SetCopied(copied)

			if onWritten != nil {
				onWritten(isTotalKnown, total, copied, written, percent)
			}
		},
		// On stop.
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error) {
			// Set number of bytes copied for the task.
			t.SetCopied(copied)

			if onStop != nil {
				onStop(isTotalKnown, total, copied, written, percent, cause)
			}
		},
		// On ok.
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			// Set number of bytes copied for the task.
			t.SetCopied(copied)

			if onOK != nil {
				onOK(isTotalKnown, total, copied, written, percent)
			}

		},
		// On error.
		func(err error) {
			if onError != nil {
				onError(err)
			}
		},
	)
}
