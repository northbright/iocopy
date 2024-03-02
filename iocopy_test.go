package iocopy_test

import (
	"context"
	"crypto/sha256"
	"encoding"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/northbright/iocopy"
)

// getRespAndSize returns the HTTP response and size of the remote file.
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

	// Check the status code.
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		log.Printf("status code is not 200 or 206: %v", resp.StatusCode)
		return nil, 0, fmt.Errorf("status code is not 200 or 206")
	}

	// Get the remote file size.
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
		written uint64
	)

	// Example of iocopy.Start().
	// ----------------------------------------------------------------------
	// It reads a remote file and calculates its SHA-256 hash.
	// It shows how to read events and process them from the event channel.
	// ----------------------------------------------------------------------

	// URL of the remote file.
	// SHA-256: 9e2f2a4031b215922aa21a3695e30bbfa1f7707597834287415dbc862c6a3251
	downloadURL := "https://golang.google.cn/dl/go1.20.1.darwin-amd64.pkg"

	// Do HTTP request and get the HTTP response body and the content length.
	resp, _, err := getRespAndSize(downloadURL, 0)
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
	// Read from response.Body and write to an hash.Hash to compute the hash.
	ch := iocopy.Start(
		// Context
		ctx,
		// Writer(dst)
		hash,
		// Reader(src)
		resp.Body,
		// Buffer size
		16*1024*1024,
		// Interval to report the number of bytes copied
		80*time.Millisecond)

	log.Printf("Example of Start(): IO copy gouroutine started.")

	// Read the events from the channel.
	for event := range ch {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			// n bytes have been written successfully.
			written = ev.Written()
			log.Printf("on EventWritten: %d bytes written", written)

		case *iocopy.EventError:
			// an error occured.
			log.Printf("on EventError: %v", ev.Err())

		case *iocopy.EventOK:
			// IO copy succeeded.
			written = ev.Written()
			log.Printf("on EventOK: %d bytes written", written)

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
	log.Printf("Example of Start(): IO copy gouroutine exited and the event channel is closed")

	// Output:
	// SHA-256:
	// 9e2f2a4031b215922aa21a3695e30bbfa1f7707597834287415dbc862c6a3251
}

func ExampleStartWithProgress() {
	var (
		nBytesToCopy uint64 = 0
		nBytesCopied uint64 = 0
		written      uint64
		state        []byte
	)

	// Example of StartWithProgress() - Part I.
	// ----------------------------------------------------------------------
	// It does the same thing as Example of Start().
	// It shows:
	// (1). how to stop the IO copy and save the state.
	// (2). how to process the EventProgress event.
	// ----------------------------------------------------------------------

	// URL of the remote file.
	// SHA-256: 9e2f2a4031b215922aa21a3695e30bbfa1f7707597834287415dbc862c6a3251
	downloadURL := "https://golang.google.cn/dl/go1.20.1.darwin-amd64.pkg"

	// Do HTTP request and get the HTTP response body and the content length.
	resp, nBytesToCopy, err := getRespAndSize(downloadURL, 0)
	if err != nil {
		log.Printf("getRespAndSize() error: %v", err)
		return
	}

	defer resp.Body.Close()

	// Create a hash.Hash for SHA-256.
	// hash.Hash is an io.Writer.
	hash := sha256.New()

	// create a context with timeout to emulate the users' cancalation of the IO copy.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start a goroutine to do IO copy.
	// Read from response.Body and write to an hash.Hash to compute the hash.
	ch := iocopy.StartWithProgress(
		// Context
		ctx,
		// Writer(dst)
		hash,
		// Reader(src)
		resp.Body,
		// Buffer size
		16*1024*1024,
		// Interval to report the number of bytes copied
		80*time.Millisecond,
		// Number of bytes to copy
		nBytesToCopy,
		// Number of bytes copied
		nBytesCopied)

	log.Printf("Example of StartWithProgress() - Part I: IO copy gouroutine started.")

	// Read the events from the channel.
	for event := range ch {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			// n bytes have been written successfully.
			written = ev.Written()
			log.Printf("on EventWritten: %d bytes written", written)

		case *iocopy.EventProgress:
			// Get EventWritten from EventProgress
			written = ev.Written()
			log.Printf("on EventProgress: current: %d/%d(%.2f%%) written, total: %d/%d written(%.2f%%)",
				written,
				nBytesToCopy,
				ev.CurrentPercent(),
				nBytesCopied+written,
				nBytesToCopy+nBytesCopied,
				ev.TotalPercent())

		case *iocopy.EventStop:
			// Context is canceled or
			// context's deadline exceeded.
			// Save the number of bytes copied.
			written = ev.Written()
			nBytesCopied += written

			log.Printf("on EventStop: %v, %d bytes written", ev.Err(), written)

			// Save the state of hash.
			marshaler, _ := hash.(encoding.BinaryMarshaler)
			state, _ = marshaler.MarshalBinary()

			log.Printf("Example of StartWithProgress() - Part I: IO copy is stopped. written: %d, nBytesCopied: %d, saved hash state: %X", written, nBytesCopied, state)

		case *iocopy.EventError:
			// an error occured.
			log.Printf("on EventError: %v", ev.Err())

		case *iocopy.EventOK:
			// IO copy succeeded.
			written = ev.Written()
			log.Printf("on EventOK: %d bytes written", written)

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
	log.Printf("Example of StartWithProgress() - Part I: IO copy gouroutine exited and the event channel is closed")

	// Example of StartWithProgress() - Part II.
	// ----------------------------------------------------------------------
	// It shows how to resume the IO copy with the saved state.
	// ----------------------------------------------------------------------

	// Do HTTP request and get the HTTP response body and the content length.
	resp2, nBytesToCopy, err := getRespAndSize(downloadURL, written)
	if err != nil {
		log.Printf("getRespAndSize() error: %v", err)
		log.Printf("it seems IO copy is done before timeout")
		return
	}

	defer resp2.Body.Close()

	// Create a hash.Hash for SHA-256.
	// hash.Hash is an io.Writer.
	hash2 := sha256.New()

	// Load the saved state.
	unmarshaler, _ := hash2.(encoding.BinaryUnmarshaler)
	unmarshaler.UnmarshalBinary(state)

	// Create a context.
	ctx2 := context.Background()

	// Start a goroutine to do IO copy.
	// Read from response.Body and write to an hash.Hash to compute the hash.
	ch2 := iocopy.StartWithProgress(
		// Context
		ctx2,
		// Writer(dst)
		hash2,
		// Reader(src)
		resp2.Body,
		// Buffer size
		16*1024*1024,
		// Interval to report the number of bytes copied
		80*time.Millisecond,
		// Number of bytes to copy
		nBytesToCopy,
		// Number of bytes copied
		nBytesCopied)

	log.Printf("Example of StartWithProgress() - Part II: IO copy gouroutine started.")

	// Read the events from the channel.
	for event := range ch2 {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			// n bytes have been written successfully.
			written = ev.Written()
			log.Printf("on EventWritten: %d bytes written", written)

		case *iocopy.EventProgress:
			written = ev.Written()
			log.Printf("on EventProgress: current: %d/%d(%.2f%%) written, total: %d/%d written(%.2f%%)",
				written,
				nBytesToCopy,
				ev.CurrentPercent(),
				nBytesCopied+written,
				nBytesToCopy+nBytesCopied,
				ev.TotalPercent())

		case *iocopy.EventError:
			// an error occured.
			log.Printf("on EventError: %v", ev.Err())

		case *iocopy.EventOK:
			// IO copy succeeded.
			written = ev.Written()
			log.Printf("on EventOK: %d bytes written", written)

			// Get the final SHA-256 checksum of the remote file.
			checksum := hash2.Sum(nil)
			fmt.Printf("SHA-256:\n%x\n", checksum)
		}
	}

	// The event channel will be closed after:
	// (1). iocopy.EventError received.
	// (2). iocopy.EventStop received.
	// (3). iocopy.EventOK received.
	// The for-range loop exits when the channel is closed.
	log.Printf("Example of StartWithProgress() - Part II: IO copy gouroutine exited and the event channel is closed")

	// Output:
	// SHA-256:
	// 9e2f2a4031b215922aa21a3695e30bbfa1f7707597834287415dbc862c6a3251
}

func ExampleCopyFile() {
	ctx := context.Background()
	dst := filepath.Join(os.TempDir(), "iocopy.go")
	src := "iocopy.go"
	bufSize := iocopy.DefaultBufSize

	log.Printf("dst: %v", dst)

	copied, err := iocopy.CopyFile(
		ctx,
		dst,
		src,
		bufSize,
		func(written, total uint64, percent float32) {
			log.Printf("on progress: %d/%d(%.2f%%) bytes written", written, total, percent)
		})

	if err != nil {
		log.Printf("CopyFile() error: %v", err)
		return
	}

	log.Printf("CopyFile() succeed, %d bytes copied.", copied)

	// Output:
}
