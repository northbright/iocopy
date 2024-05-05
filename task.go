package iocopy

import (
	"context"
	"encoding/json"
	"io"
)

type OnWritten func(isTotalKnown bool, total, copied, written uint64, percent float32)

type OnStop func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, data []byte)

type OnOK func(isTotalKnown bool, total, copied, written uint64, percent float32)

type OnError func(err error)

type Task interface {
	setCopied(copied uint64)
	writer() io.Writer
	reader() io.Reader
	total() (bool, uint64)
	copied() uint64
	json.Marshaler
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

			data, err := t.MarshalJSON()
			if err != nil {
				if onError != nil {
					onError(err)
				}
			} else {
				if onStop != nil {
					onStop(
						ew.IsTotalKnown(),
						ew.Total(),
						ew.Copied(),
						ew.Written(),
						ew.Percent(),
						ev.Cause(),
						data,
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
