package iocopy_test

import (
	"context"
	"crypto/sha256"
	"encoding"
	"fmt"
	"log"
	"time"

	"github.com/northbright/httputil"
	"github.com/northbright/iocopy"
)

/*
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
	resp, isTotalKnown, size, isRangeSupported, err := httputil.GetResp(downloadURL)
	if err != nil {
		log.Printf("GetResp() error: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("GetResp() for %v OK", downloadURL)
	log.Printf("is total size known: %v, size: %d, is range supported: %v",
		isTotalKnown,
		size,
		isRangeSupported)

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
			log.Printf("on EventWritten: %d/%d bytes written", written, size)

		case *iocopy.EventError:
			// an error occured.
			log.Printf("on EventError: %v", ev.Err())

		case *iocopy.EventOK:
			// IO copy succeeded.
			written = ev.Written()
			log.Printf("on EventOK: %d/%d bytes written", written, size)

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
*/

func ExampleStart() {
	var (
		total  uint64 = 0
		copied uint64 = 0
		state  []byte
	)

	// Example of Start() - Part I.
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
	resp, isTotalKnown, total, isRangeSupported, err := httputil.GetResp(downloadURL)
	if err != nil {
		log.Printf("GetResp() error: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("GetResp() for %v OK", downloadURL)
	log.Printf("is total size known: %v, size: %d, is range supported: %v",
		isTotalKnown,
		total,
		isRangeSupported)

	// Create a hash.Hash for SHA-256.
	// hash.Hash is an io.Writer.
	hash := sha256.New()

	// create a context with timeout to emulate the users' cancalation of the IO copy.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start a goroutine to do IO copy.
	// Read from response.Body and write to an hash.Hash to compute the hash.
	ch := iocopy.Start(
		// context
		ctx,
		// writer(dst)
		hash,
		// reader(src)
		resp.Body,
		// buffer size
		16*1024*1024,
		// interval to report the number of bytes written
		80*time.Millisecond,
		// is total bytes to copy known
		isTotalKnown,
		// total number of bytes to copy
		total,
		// the number of bytes copied
		copied)

	log.Printf("Example of Start() - Part I: IO copy gouroutine started.")

	// Read the events from the channel.
	for event := range ch {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			// n bytes have been written successfully.
			log.Printf("on EventWritten: %d bytes written, %d/%d(%.2f%%) bytes copied",
				ev.Written(),
				ev.Copied(),
				ev.Total(),
				ev.Percent(),
			)

		case *iocopy.EventStop:
			// Context is canceled or
			// context's deadline exceeded.
			// Set the number of bytes copied.
			ew := ev.EventWritten()
			copied = ew.Copied()

			// Save the state of hash.
			marshaler, _ := hash.(encoding.BinaryMarshaler)
			state, _ = marshaler.MarshalBinary()

			log.Printf("on EvnetStop(%v): %d bytes written, %d/%d(%.2f%%) copied. Saved hash state: %X",
				ev.Cause(),
				ew.Written(),
				ew.Copied(),
				ew.Total(),
				ew.Percent(),
				state,
			)

		case *iocopy.EventError:
			// an error occured.
			log.Printf("on EventError: %v", ev.Err())

		case *iocopy.EventOK:
			// Should not receive EventOK this time.
		}
	}

	// The event channel will be closed after:
	// (1). iocopy.EventError received.
	// (2). iocopy.EventStop received.
	// (3). iocopy.EventOK received.
	// The for-range loop exits when the channel is closed.
	log.Printf("Example of Start() - Part I: IO copy gouroutine exited and the event channel is closed")

	// Example of Start() - Part II.
	// ----------------------------------------------------------------------
	// It shows how to resume the IO copy with the saved state.
	// ----------------------------------------------------------------------

	// Do HTTP request and get the HTTP response body and the size of partial content.
	resp2, sizeOfPartialContent, err := httputil.GetRespOfRangeStart(downloadURL, copied)
	if err != nil {
		log.Printf("GetRespOfRangeStart() error: %v", err)
		log.Printf("it seems IO copy is done before timeout")
		return
	}
	defer resp2.Body.Close()

	log.Printf("GetRespOfRangeStart(): 'bytes=%d-' for %v OK", copied, downloadURL)
	log.Printf("size of partial content: %d", sizeOfPartialContent)

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
	ch2 := iocopy.Start(
		// context
		ctx2,
		// writer(dst)
		hash2,
		// reader(src)
		resp2.Body,
		// buffer size
		16*1024*1024,
		// interval to report the number of bytes written
		80*time.Millisecond,
		// is total bytes to copy known
		isTotalKnown,
		// total number of bytes to copy
		total,
		// number of bytes copied
		copied)

	log.Printf("Example of Start() - Part II: IO copy gouroutine started.")

	// Read the events from the channel.
	for event := range ch2 {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			// n bytes have been written successfully.
			log.Printf("on EventWritten: %d bytes written, %d/%d(%.2f%%) bytes copied",
				ev.Written(),
				ev.Copied(),
				ev.Total(),
				ev.Percent(),
			)

		case *iocopy.EventStop:
			// Should not receive EventStop this time.

		case *iocopy.EventError:
			// an error occured.
			log.Printf("on EventError: %v", ev.Err())

		case *iocopy.EventOK:
			// IO copy succeeded.
			ew := ev.EventWritten()
			log.Printf("on EventOK: %d bytes written, %d/%d(%.2f%%) bytes copied",
				ew.Written(),
				ew.Copied(),
				ew.Total(),
				ew.Percent(),
			)

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
	log.Printf("Example of Start() - Part II: IO copy gouroutine exited and the event channel is closed")

	// Output:
	// SHA-256:
	// 9e2f2a4031b215922aa21a3695e30bbfa1f7707597834287415dbc862c6a3251
}

/*
func ExampleCopyFile() {
	type CopyParam struct {
		Dst string
		Src string
		// Used to create the source file on the fly if needed.
		GenSrcFunc func(src string)
	}

	cpParams := []CopyParam{
		CopyParam{
			Dst:        filepath.Join(os.TempDir(), "iocopy.go"),
			Src:        "iocopy.go",
			GenSrcFunc: nil,
		},
		CopyParam{
			Dst: filepath.Join(os.TempDir(), "b.go"),
			Src: filepath.Join(os.TempDir(), "a.go"),
			GenSrcFunc: func(src string) {
				// Create a 0-byte file
				f, _ := os.Create(src)
				f.Close()
			},
		},
	}

	ctx := context.Background()
	bufSize := iocopy.DefaultBufSize

	for _, param := range cpParams {
		if param.GenSrcFunc != nil {
			param.GenSrcFunc(param.Src)
		}

		dst := param.Dst
		src := param.Src

		log.Printf("CopyFile() started. dst: %s, src: %s", dst, src)

		// Copy src to dst.
		copied, err := iocopy.CopyFile(
			ctx,
			dst,
			src,
			bufSize,
			func(isTotalSizeUnknown bool, total, written uint64, percent float32) {
				log.Printf("on progress: %d/%d(%.2f%%) bytes written", written, total, percent)
			})

		if err != nil {
			log.Printf("CopyFile() error: %v", err)
			return
		}

		log.Printf("CopyFile() succeed, %d bytes copied.", copied)

		// Remove the file after test's done.
		os.Remove(dst)
	}

	// Output:
}

func ExampleCopyFileFS() {
	ctx := context.Background()
	dst := filepath.Join(os.TempDir(), "README.md")
	src := "README.md"
	bufSize := iocopy.DefaultBufSize

	log.Printf("CopyFileFS() started. dst: %s, src: %s", dst, src)

	// Copy src in embed.FS to dst.
	copied, err := iocopy.CopyFileFS(
		ctx,
		dst,
		embededFiles,
		src,
		bufSize,
		func(isTotalSizeUnknown bool, total, written uint64, percent float32) {
			log.Printf("on progress: %d/%d(%.2f%%) bytes written", written, total, percent)
		})

	if err != nil {
		log.Printf("CopyFileFS() error: %v", err)
		return
	}

	log.Printf("CopyFileFS() succeed, %d bytes copied.", copied)

	// Remove the file after test's done.
	os.Remove(dst)

	// Output:
}
*/
