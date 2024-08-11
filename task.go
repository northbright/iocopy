package iocopy

import (
	"context"
	"io"
)

type OnWritten func(isTotalKnown bool, total, copied, written uint64, percent float32)

type OnStop func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error)

type OnOK func(isTotalKnown bool, total, copied, written uint64, percent float32)

type OnError func(err error)

type Task interface {
	Writer() io.Writer
	Reader() io.Reader
	Total() (bool, uint64)
	Copied() uint64
	SetCopied(uint64)
}

func Do(
	ctx context.Context,
	t Task,
	bufSize uint,
	onWritten OnWritten,
	onStop OnStop,
	onOK OnOK,
	onError OnError) {
	isTotalKnown, total := t.Total()
	copied := t.Copied()

	w := t.Writer()
	wc, ok := w.(io.WriteCloser)
	if ok {
		defer wc.Close()
	}

	r := t.Reader()
	rc, ok := r.(io.ReadCloser)
	if ok {
		defer rc.Close()
	}

	ch := Start(
		ctx,
		w,
		r,
		bufSize,
		DefaultInterval,
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
