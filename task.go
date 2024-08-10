package iocopy

import (
	"context"
	"io"
)

type OnWritten func(isTotalKnown bool, total, copied, written uint64, percent float32)

type OnStop func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, state []byte)

type OnOK func(isTotalKnown bool, total, copied, written uint64, percent float32)

type OnError func(err error)

type Task interface {
	writer() io.Writer
	reader() io.Reader
	total() (bool, uint64)
	copied() uint64
	setCopied(uint64)
	state() ([]byte, error)
}

func Do(
	ctx context.Context,
	t Task,
	bufSize uint,
	onWritten OnWritten,
	onStop OnStop,
	onOK OnOK,
	onError OnError) {
	isTotalKnown, total := t.total()
	copied := t.copied()

	w := t.writer()
	wc, ok := w.(io.WriteCloser)
	if ok {
		defer wc.Close()
	}

	r := t.reader()
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

			t.setCopied(ew.Copied())

			if onStop != nil {
				state, err := t.state()
				if err != nil {
					if onError != nil {
						onError(err)
					}
				} else {
					onStop(
						ew.IsTotalKnown(),
						ew.Total(),
						ew.Copied(),
						ew.Written(),
						ew.Percent(),
						ev.Cause(),
						state,
					)
				}
			}

		case *EventOK:
			ew := ev.EventWritten()

			t.setCopied(ew.Copied())

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
