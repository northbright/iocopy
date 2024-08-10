package iocopy_test

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/northbright/iocopy"
)

func ExampleNewDownloadTask() {
	var (
		savedState []byte
	)

	dst := filepath.Join(os.TempDir(), "go1.22.2.darwin-amd64.pkg")
	url := "https://golang.google.cn/dl/go1.22.2.darwin-amd64.pkg"

	// Create a download task.
	t, err := iocopy.NewDownloadTask(
		// Destination
		dst,
		// Url
		url,
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
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, state []byte) {
			log.Printf("on stop(%v): %d/%d(%.2f%%)\nstate: %s", cause, copied, total, percent, string(state))
			// Save the state for resuming downloading.
			savedState = state
		},
		// On ok
		func(isTotalKnown bool, total, copied, written uint64, percent float32, result []byte) {
			log.Printf("on ok: %d/%d(%.2f%%)\nresult: %s", copied, total, percent, string(result))
		},
		// On error
		func(err error) {
			log.Printf("on error: %v", err)
		},
	)

	// Load the task from the saved state and resume downloading.
	t, err = iocopy.LoadDownloadTask(savedState)
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
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, state []byte) {
			log.Printf("on stop(%v): %d/%d(%.2f%%)\nstate: %s", cause, copied, total, percent, string(state))
		},
		// On ok
		func(isTotalKnown bool, total, copied, written uint64, percent float32, result []byte) {
			log.Printf("on ok: %d/%d(%.2f%%)\nresult: %s", copied, total, percent, string(result))
		},
		// On error
		func(err error) {
			log.Printf("on error: %v", err)
		},
	)

	// Remove the files after test's done.
	os.Remove(dst)

	// Output:

}

func ExampleDownload() {
	ctx := context.Background()
	dst := filepath.Join(os.TempDir(), "go1.22.2.darwin-amd64.pkg")
	url := "https://golang.google.cn/dl/go1.22.2.darwin-amd64.pkg"
	bufSize := uint(4 * 1024)

	if err := iocopy.Download(ctx, dst, url, bufSize); err != nil {
		log.Printf("Download() error: %v", err)
		return
	}

	log.Printf("Download() ok")

	// Remove the files after test's done.
	os.Remove(dst)

	// Output:
}
