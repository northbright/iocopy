package iocopy_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/northbright/iocopy"
)

func ExampleCopy() {
	// This example uses iocopy.Copy to read stream from a remote file,
	// and compute its SHA-256 checksum.
	// It uses a timeout context to emulate user cancelation to stop the calculation.
	// Then it calls iocopy.Copy again to resume the calculation.

	// SHA-256: dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
	url := "https://golang.google.cn/dl/go1.23.1.darwin-amd64.pkg"

	// Get response body(io.Reader) of the remote file.
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("http.Get() error: %v, url: %v", err, url)
	}
	defer resp.Body.Close()

	// Create a hash.Hash(io.Writer) for SHA-256.
	// hash.Hash is an io.Writer.
	hash := sha256.New()

	// Use a timeout to emulate user's cancelation.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()

	// Call iocopy.Copy to start the calculation.
	n, err := iocopy.Copy(ctx, hash, resp.Body)
	if err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			// Got error during IO copy.
			log.Printf("iocopy.Copy() error: %v", err)
			return
		}
		// Stopped.
		log.Printf("iocopy.Copy() stopped, cause: %v\nbytes copied: %v", ctx.Err(), n)
	}

	// Create a request with "range" header set.
	client := http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("http.NewRequest() error: %v", err)
		return
	}

	bytesRange := fmt.Sprintf("bytes=%d-", n)
	req.Header.Add("range", bytesRange)

	// Do HTTP request.
	resp2, err := client.Do(req)
	if err != nil {
		log.Printf("client.Do() error: %v", err)
		return
	}
	defer resp2.Body.Close()

	// Call iocopy.Copy again to resume the calculation.
	n2, err := iocopy.Copy(context.Background(), hash, resp2.Body)
	if err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			// Got error during IO copy.
			log.Printf("iocopy.Copy() error: %v", err)
			return
		}
		// Stopped.
		log.Printf("iocopy.Copy() stopped, cause: %v\nbytes copied: %v", ctx.Err(), n)
	}
	log.Printf("iocopy.Copy() done.\nbytes copied: %v, total: %v bytes copied", n2, n+n2)

	checksum := hash.Sum(nil)
	fmt.Printf("SHA-256:\n%x\n", checksum)

	// Output:
	// SHA-256:
	// dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
}
