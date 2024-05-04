package iocopy_test

import (
	"context"
	"log"
	"time"

	"github.com/northbright/iocopy"
)

func ExampleNewDownloadTask() {
	var (
		savedData []byte
	)

	// Create a download task.
	t, err := iocopy.NewDownloadTask(
		// Dst
		"/tmp/go1.22.2.darwin-amd64.pkg",
		// Url
		"https://golang.google.cn/dl/go1.22.2.darwin-amd64.pkg",
	)
	if err != nil {
		log.Printf("NewDownloadTask() error: %v", err)
		return
	}

	// Use a timeout to emulate that users stop the downloading.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()

	bufSize := uint(64 * 1024)

	// Do the task and block caller's go routine until the io copy go routine is done.
	iocopy.Do(
		// Context
		ctx,
		// Task
		t,
		// Buffer size
		bufSize,
		// On bytes written
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on written: %d/%d(%.2f%%)", copied, total, percent)
		},
		// On stop
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, data []byte) {
			log.Printf("on stop(%v): %d/%d(%.2f%%), data: %s", cause, copied, total, percent, string(data))
			// Save data for resuming downloading.
			savedData = data
		},
		// On ok
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on ok: %d/%d(%.2f%%)", copied, total, percent)
		},
		// On error
		func(err error) {
			log.Printf("on error: %v", err)
		},
	)

	// Load saved data to resume downloading.
	t, err = iocopy.LoadDownloadTask(savedData)
	if err != nil {
		log.Printf("LoadDownloadTask() error: %v", err)
		return
	}

	ctx = context.Background()

	// Do the task and block caller's go routine until the io copy go routine is done.
	iocopy.Do(
		// Context
		ctx,
		// Task
		t,
		// Buffer size
		bufSize,
		// On bytes written
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on written: %d/%d(%.2f%%)", copied, total, percent)
		},
		// On stop
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, data []byte) {
			log.Printf("on stop(%v): %d/%d(%.2f%%), data: %s", cause, copied, total, percent, string(data))
		},
		// On ok
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on ok: %d/%d(%.2f%%)", copied, total, percent)
		},
		// On error
		func(err error) {
			log.Printf("on error: %v", err)
		},
	)

	// Output:
}
