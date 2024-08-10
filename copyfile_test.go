package iocopy_test

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/northbright/iocopy"
)

var (
	//go:embed README.md
	embededFiles embed.FS
)

func ExampleNewCopyFileTask() {
	var (
		savedState []byte
	)

	dst := filepath.Join(os.TempDir(), "go1.22.2.darwin-amd64.pkg")
	url := "https://golang.google.cn/dl/go1.22.2.darwin-amd64.pkg"

	// Download a file.
	err := iocopy.Download(
		// Context
		context.Background(),
		// Destination file
		dst,
		// URL of file to download
		url,
		// Buffer Size
		iocopy.DefaultBufSize,
	)

	if err != nil {
		log.Printf("Download() error: %v", err)
		return
	}

	// Copy the downloaded file to another file.
	src := dst
	dst = filepath.Join(os.TempDir(), "go.pkg")

	// Create a copy file task.
	t, err := iocopy.NewCopyFileTask(
		// Destination file
		dst,
		// Source file
		src,
	)
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
			// Save the state to resume copying.
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

	// Load the task from the saved state and resume copying.
	t, err = iocopy.LoadCopyFileTask(savedState)
	if err != nil {
		log.Printf("LoadCopyFileTask() error: %v", err)
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
	os.Remove(src)

	// Output:
}

func ExampleCopyFile() {
	ctx := context.Background()
	dst := filepath.Join(os.TempDir(), "README.md")
	src := "README.md"
	bufSize := uint(4 * 1024)

	err := iocopy.CopyFile(
		// Context
		ctx,
		// Destination file
		dst,
		// Source file
		src,
		// Buffer size
		bufSize,
	)

	if err != nil {
		log.Printf("CopyFile() error: %v", err)
		return
	}

	log.Printf("CopyFile() ok")

	// Remove the files after test's done.
	os.Remove(dst)

	// Output:
}

func ExampleNewCopyFileFromFSTask() {
	ctx := context.Background()
	dst := filepath.Join(os.TempDir(), "README.md")
	src := "README.md"
	bufSize := uint(4 * 1024)

	t, err := iocopy.NewCopyFileFromFSTask(dst, embededFiles, src)
	if err != nil {
		log.Printf("NewCopyFileFromFSTask() error: %v", err)
		return
	}

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

func ExampleCopyFileFromFS() {
	ctx := context.Background()
	dst := filepath.Join(os.TempDir(), "README.md")
	src := "README.md"
	bufSize := uint(4 * 1024)

	err := iocopy.CopyFileFromFS(
		// Context
		ctx,
		// Destination file
		dst,
		// File system contains source file
		embededFiles,
		// Source file
		src,
		// Buffer size
		bufSize,
	)

	if err != nil {
		log.Printf("CopyFileFromFS() error: %v", err)
		return
	}

	log.Printf("CopyFileFromFS() ok")

	// Remove the files after test's done.
	os.Remove(dst)

	// Output:
}
