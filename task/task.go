package task

import (
	"context"
	"encoding/json"
	"io"

	"github.com/northbright/iocopy"
)

type OnWritten func(isTotalKnown bool, total, copied, written uint64, percent float32)

type OnStop func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, data []byte)

type OnOK func(isTotalKnown bool, total, copied, written uint64, percent float32)

type OnError func(err error)

type Task interface {
	Total() (bool, uint64)
	Copied() uint64
	SetCopied(copied uint64)
	Writer() io.Writer
	Reader() io.Reader
	json.Marshaler
	json.Unmarshaler
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

	ch := iocopy.Start(
		ctx,
		w,
		r,
		bufSize,
		iocopy.DefaultInterval,
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

			t.SetCopied(ew.Copied())

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

		case *iocopy.EventOK:
			ew := ev.EventWritten()

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
