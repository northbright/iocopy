package iocopy_test

import (
	"context"
	"crypto/sha256"
	"encoding"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/northbright/iocopy"
)

// computePercent returns the percentage.
func computePercent(total, written uint64, done bool) float32 {
	if done {
		// Return 100 percent when copy is done,
		// even if written and total are both 0.
		return 100
	}

	if total == 0 {
		return 0
	}

	return float32(float64(written) / (float64(total) / float64(100)))
}

// getRespAndSize returns the HTTP response and size of remote file.
func getRespAndSize(remoteURL string, written uint64) (*http.Response, uint64, error) {
	// Create a HTTP client.
	client := http.Client{}
	req, err := http.NewRequest("GET", remoteURL, nil)
	if err != nil {
		return nil, 0, err
	}

	// Set range header to resume downloading if need.
	if written > 0 {
		bytesRange := fmt.Sprintf("bytes=%d-", written)
		req.Header.Add("range", bytesRange)
	}

	// Do HTTP request.
	// resp.Body(io.ReadCloser) will be closed
	// when Hasher.Close is called.
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	// Check status code.
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		log.Printf("status code is not 200 or 206")
		return nil, 0, err
	}

	// Get remote file size.
	contentLength := resp.Header.Get("Content-Length")
	total, _ := strconv.ParseUint(contentLength, 10, 64)
	if total <= 0 {
		log.Printf("Content-Length <= 0: %v", total)
		return nil, 0, err
	}

	return resp, total, nil
}

func ExampleStart() {
	var (
		total      uint64
		written    uint64
		oldWritten uint64
		percent    float32
		state      []byte
	)

	// Example 1
	// ----------------------------------------------------------------------
	// It reads a remote file and calculates its SHA-256 hash.
	// It shows how to read events and process them from the event channel.
	// ----------------------------------------------------------------------

	// URL of remote file.
	// SHA-256: 9e2f2a4031b215922aa21a3695e30bbfa1f7707597834287415dbc862c6a3251
	downloadURL := "https://golang.google.cn/dl/go1.20.1.darwin-amd64.pkg"

	// Do HTTP request and get the HTTP response body and content length.
	resp, total, err := getRespAndSize(downloadURL, 0)
	if err != nil {
		log.Printf("getRespAndSize() error: %v", err)
		return
	}

	defer resp.Body.Close()

	// Create a hash.Hash for SHA-256.
	// hash.Hash is an io.Writer.
	hash := sha256.New()

	// create a context.
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
		200*time.Millisecond)

	log.Printf("Example 1: IO copy gouroutine started.")

	// Read the events from the channel.
	for event := range ch {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			// n bytes have been written successfully.
			// Get the count of bytes.
			written = ev.Written()
			percent = computePercent(total, written, false)
			log.Printf("on EventWritten: %v/%v bytes written(%.2f%%)", written, total, percent)

		case *iocopy.EventError:
			// an error occured.
			// Get the error.
			log.Printf("on EventError: %v", ev.Err())

		case *iocopy.EventOK:
			// IO copy succeeded.
			// Get EventWritten from EventOK.
			ew := ev.EventWritten()

			// Get the number of written bytes and percent.
			written = ew.Written()
			percent = computePercent(total, written, true)
			log.Printf("on EventOK: %v/%v bytes written(%.2f%%)", written, total, percent)

			// Get the final SHA-256 checksum of the remote file.
			checksum := hash.Sum(nil)
			fmt.Printf("SHA-256:\n%x\n", checksum)
		}
	}

	// The event channel will be closed after:
	// (1). iocopy.EventError received.
	// (2). iocopy.EventStop received.
	// (3). iocopy.EventOK received.
	// The for-range loop exits when the channel is closed.
	log.Printf("Example 1: IO copy gouroutine exited and the event channel is closed")

	// Example 2 - Part I.
	// ----------------------------------------------------------------------
	// It does the same thing as Example 1.
	// It shows how to stop the IO copy and save the state.
	// ----------------------------------------------------------------------

	// Do HTTP request and get the HTTP response body and content length.
	resp2, total, err := getRespAndSize(downloadURL, 0)
	if err != nil {
		log.Printf("getRespAndSize() error: %v", err)
		return
	}

	defer resp2.Body.Close()

	// Create a hash.Hash for SHA-256.
	// hash.Hash is an io.Writer.
	hash2 := sha256.New()

	// create a context with timeout to emulate the users' cancalation of the IO copy.
	ctx2, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	// Start a goroutine to do IO copy.
	// Read from response.Body and write to hash.Hash to compute hash.
	ch2 := iocopy.Start(
		// Context
		ctx2,
		// Writer(dst)
		hash2,
		// Reader(src)
		resp2.Body,
		// Buffer size
		16*1024*1024,
		// Interval to report written bytes
		200*time.Millisecond)

	log.Printf("Example 2 - Part I: IO copy gouroutine started.")

	// Read the events from the channel.
	for event := range ch2 {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			// n bytes have been written successfully.
			// Get the count of bytes.
			written = ev.Written()
			percent = computePercent(total, written, false)
			log.Printf("on EventWritten: %v/%v bytes written(%.2f%%)", written, total, percent)

		case *iocopy.EventStop:
			// Context is canceled or
			// context's deadline exceeded.
			// Get EventWritten from EventStop.
			ew := ev.EventWritten()

			// Get the number of written bytes and percent.
			written = ew.Written()

			// Save the number of written bytes and total bytes.
			oldWritten = written

			percent = computePercent(total, written, false)

			log.Printf("on EventStop: %v, %v/%v bytes written(%.2f%%)", ev.Err(), written, total, percent)

			// Save the state of hash.
			marshaler, _ := hash2.(encoding.BinaryMarshaler)
			state, _ = marshaler.MarshalBinary()

			log.Printf("IO copy is stopped. written: %d, save hash state: %X", written, state)

		case *iocopy.EventError:
			// an error occured.
			// Get the error.
			log.Printf("on EventError: %v", ev.Err())

		case *iocopy.EventOK:
			// IO copy succeeded.
			// Get EventWritten from EventOK.
			ew := ev.EventWritten()

			// Get the number of written bytes and percent.
			written = ew.Written()
			percent = computePercent(total, written, true)
			log.Printf("on EventOK: %v/%v bytes written(%.2f%%)", written, total, percent)

			// Get the final SHA-256 checksum of the remote file.
			checksum := hash.Sum(nil)
			fmt.Printf("SHA-256:\n%x\n", checksum)
		}
	}

	// The event channel will be closed after:
	// (1). iocopy.EventError received.
	// (2). iocopy.EventStop received.
	// (3). iocopy.EventOK received.
	// The for-range loop exits when the channel is closed.
	log.Printf("Example 2 - Part I: IO copy gouroutine exited and the event channel is closed")

	// Example 2 - Part II.
	// ----------------------------------------------------------------------
	// It shows how to resume the IO copy with the saved state.
	// ----------------------------------------------------------------------

	// Do HTTP request and get the HTTP response body and content length.
	resp3, total, err := getRespAndSize(downloadURL, written)
	if err != nil {
		log.Printf("getRespAndSize() error: %v", err)
		return
	}

	defer resp3.Body.Close()

	// Update total.
	// "total" is the total size of left content(partial) to copy.
	total += oldWritten

	// Create a hash.Hash for SHA-256.
	// hash.Hash is an io.Writer.
	hash3 := sha256.New()

	// Load the saved state.
	unmarshaler, _ := hash3.(encoding.BinaryUnmarshaler)
	unmarshaler.UnmarshalBinary(state)

	// Create a context.
	ctx3 := context.Background()

	// Start a goroutine to do IO copy.
	// Read from response.Body and write to hash.Hash to compute hash.
	ch3 := iocopy.Start(
		// Context
		ctx3,
		// Writer(dst)
		hash3,
		// Reader(src)
		resp3.Body,
		// Buffer size
		16*1024*1024,
		// Interval to report written bytes
		200*time.Millisecond)

	log.Printf("Example 2 - Part II: IO copy gouroutine started.")

	// Read the events from the channel.
	for event := range ch3 {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			// n bytes have been written successfully.
			// Get the count of bytes and update the total number of written bytes.
			written = oldWritten + ev.Written()
			percent = computePercent(total, written, false)
			log.Printf("on EventWritten: %v/%v bytes written(%.2f%%)", written, total, percent)

		case *iocopy.EventError:
			// an error occured.
			// Get the error.
			log.Printf("on EventError: %v", ev.Err())

		case *iocopy.EventOK:
			// IO copy succeeded.
			// Get EventWritten from EventOK.
			ew := ev.EventWritten()

			// Get the number of written bytes and percent.z
			written = oldWritten + ew.Written()
			percent = computePercent(total, written, true)
			log.Printf("on EventOK: %v/%v bytes written(%.2f%%)", written, total, percent)

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
	log.Printf("Example 2 - Part II: IO copy gouroutine exited and the event channel is closed")

	// Output:
	// SHA-256:
	// 9e2f2a4031b215922aa21a3695e30bbfa1f7707597834287415dbc862c6a3251
	// SHA-256:
	// 9e2f2a4031b215922aa21a3695e30bbfa1f7707597834287415dbc862c6a3251
}
