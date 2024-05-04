package iocopy_test

import (
	"context"
	"log"
	"time"

	"github.com/northbright/iocopy"
)

func ExampleNewCopyFileTask() {
	var (
		savedData []byte
	)

	// Download a file.
	err := iocopy.Download(
		// Context
		context.Background(),
		// Dst
		"/tmp/go1.22.2.darwin-amd64.pkg",
		// Url
		"https://golang.google.cn/dl/go1.22.2.darwin-amd64.pkg",
		// Buffer Size
		iocopy.DefaultBufSize,
	)

	if err != nil {
		log.Printf("Download() error: %v", err)
		return
	}

	// Create a copy file task.
	t, err := iocopy.NewCopyFileTask("/tmp/go.pkg", "/tmp/go1.22.2.darwin-amd64.pkg")
	if err != nil {
		log.Printf("NewCopyFileTask() error: %v", err)
		return
	}

	// Use a timeout to emulate that users stop the copy.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()

	bufSize := uint(4 * 1024)

	// Do the task and block caller's go routine until the io copy go routine is done.
	iocopy.Do(
		ctx,
		t,
		bufSize,
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on written: %d/%d(%.2f%%)", copied, total, percent)
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, data []byte) {
			log.Printf("on stop(%v): %d/%d(%.2f%%), data: %s", cause, copied, total, percent, string(data))
			// Save data to resume copying.
			savedData = data
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on ok: %d/%d(%.2f%%)", copied, total, percent)
		},
		func(err error) {
			log.Printf("on error: %v", err)
		},
	)

	// Load saved data to resume copying.
	t, err = iocopy.LoadCopyFileTask(savedData)
	if err != nil {
		log.Printf("LoadCopyFileTask() error: %v", err)
		return
	}

	ctx = context.Background()

	// Do the task and block caller's go routine until the io copy go routine is done.
	iocopy.Do(
		ctx,
		t,
		bufSize,
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on written: %d/%d(%.2f%%)", copied, total, percent)
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, data []byte) {
			log.Printf("on stop(%v): %d/%d(%.2f%%), data: %s", cause, copied, total, percent, string(data))
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on ok: %d/%d(%.2f%%)", copied, total, percent)
		},
		func(err error) {
			log.Printf("on error: %v", err)
		},
	)

	// Output:
}
