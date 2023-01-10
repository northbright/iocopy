package iocopy_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/northbright/iocopy"
)

func ExampleStart() {
	// Example of iocopy.Start()
	// It reads a remote file and calculates its SHA-256 hash.
	// It shows how to read events and process them from the event channel.

	// URL of Ubuntu release.
	// SHA-256:
	// 10f19c5b2b8d6db711582e0e27f5116296c34fe4b313ba45f9b201a5007056cb
	downloadURL := "https://www.releases.ubuntu.com/jammy/ubuntu-22.04.1-live-server-amd64.iso"

	// Do HTTP request and get the response.
	resp, err := http.Get(downloadURL)
	if err != nil {
		log.Printf("http.Get() error: %v", err)
		return
	}
	// Check status code.
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		log.Printf("status code is not 200 or 206")
		return
	}

	// response.Body is an io.ReadCloser.
	// Do not forget to close the body.
	// iocopy.Start can also close the io.ReadCloser on goroutine exit when
	// tryClosingReaderOnExit is set to true.
	// Uncomment below line if tryClosingReaderOnExit is set to false when
	// call iocopy.Start.
	// defer resp.Body.Close()

	// Get remote file size.
	contentLength := resp.Header.Get("Content-Length")
	total, _ := strconv.ParseInt(contentLength, 10, 64)
	if total <= 0 {
		log.Printf("Content-Length <= 0: %v", total)
		return
	}

	// Create a hash.Hash for SHA-256.
	// hash.Hash is an io.Writer.
	hash := sha256.New()

	// create a context.
	// It can be created by context.WithCancel, context.WithDeadline,
	// context.WithTimeout...
	// You may test timeout context.
	// The IO copy should fail and the channel will receive an EventError:
	// context deadline exceeded.
	// ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	// defer cancel()

	// Use background context by default.
	// IO copy should succeed and the channel will receive an EventOK.
	ctx := context.Background()

	// Start a goroutine to do IO copy.
	// Read from response.Body and write to hash.Hash to compute hash.
	ch := iocopy.Start(
		// Context
		ctx,
		// Writer(dst)
		hash,
		// Reader(src)
		resp.Body,
		// Buffer size
		16*1024*1024,
		// Interval to report written bytes
		500*time.Millisecond,
		// Try closing reader on exit
		true)

	// Read the events from the channel.
	for event := range ch {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			// n bytes have been written successfully.
			// Get the count of bytes.
			n := ev.Written()
			percent := float32(float64(n) / (float64(total) / float64(100)))
			log.Printf("on EventWritten: %v/%v bytes written(%.2f%%)", n, total, percent)

		case *iocopy.EventStop:
			// Context is canceled or
			// context's deadline exceeded.
			log.Printf("on EventStop: %v", ev.Err())

		case *iocopy.EventError:
			// an error occured.
			// Get the error.
			log.Printf("on EventError: %v", ev.Err())

		case *iocopy.EventOK:
			// IO copy succeeded.
			// Get the total count of written bytes.
			n := ev.Written()
			percent := float32(float64(n) / (float64(total) / float64(100)))
			log.Printf("on EventOK: %v/%v bytes written(%.2f%%)", n, total, percent)

			// Get the final SHA-256 checksum of the remote file.
			checksum := hash.Sum(nil)
			fmt.Printf("SHA-256:\n%x", checksum)
		}
	}

	// The event channel will be closed after:
	// (1). iocopy.EventError received.
	// (2). iocopy.EventStop received.
	// (3). iocopy.EventOK received.
	// The for-range loop exits when the channel is closed.
	log.Printf("IO copy gouroutine exited and the event channel is closed")

	// Output:
	// SHA-256:
	// 10f19c5b2b8d6db711582e0e27f5116296c34fe4b313ba45f9b201a5007056cb
}
