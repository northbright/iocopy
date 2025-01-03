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
		return
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
	} else {
		// IO copy done after first call, no need to resume.
		log.Printf("iocopy.Copy() OK, %v bytes copied", n)
		checksum := hash.Sum(nil)
		fmt.Printf("SHA-256:\n%x\n", checksum)
		return
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
	log.Printf("call iocopy.Copy() again to resume the calculation")

	n2, err := iocopy.Copy(context.Background(), hash, resp2.Body)
	if err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			// Got error during IO copy.
			log.Printf("iocopy.Copy() error: %v", err)
			return
		}
		// Stopped.
		log.Printf("iocopy.Copy() stopped, cause: %v\nbytes copied: %v", ctx.Err(), n)
	} else {
		log.Printf("iocopy.Copy() OK, %v bytes copied", n2)
	}

	log.Printf("total: %v bytes copied", n+n2)

	checksum := hash.Sum(nil)
	fmt.Printf("SHA-256:\n%x\n", checksum)

	// Output:
	// SHA-256:
	// dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
}

func ExampleCopyBuffer() {
	// This example uses iocopy.CopyBuffer to read stream from a remote file,
	// and compute its SHA-256 checksum.
	// It uses a timeout context to emulate user cancelation to stop the calculation.
	// Then it calls iocopy.CopyBuffer again to resume the calculation.

	// SHA-256: dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
	url := "https://golang.google.cn/dl/go1.23.1.darwin-amd64.pkg"

	// Get response body(io.Reader) of the remote file.
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("http.Get() error: %v, url: %v", err, url)
		return
	}
	defer resp.Body.Close()

	// Create a hash.Hash(io.Writer) for SHA-256.
	// hash.Hash is an io.Writer.
	hash := sha256.New()

	// Use a timeout to emulate user's cancelation.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()

	// Create a buffer.
	buf := make([]byte, 1024*640)

	// Call iocopy.CopyBuffer to start the calculation.
	n, err := iocopy.CopyBuffer(ctx, hash, resp.Body, buf)
	if err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			// Got error during IO copy.
			log.Printf("iocopy.CopyBuffer() error: %v", err)
			return
		}
		// Stopped.
		log.Printf("iocopy.CopyBuffer() stopped, cause: %v\nbytes copied: %v", ctx.Err(), n)
	} else {
		// IO copy done after first call, no need to resume.
		log.Printf("iocopy.CopyBuffer() OK, %v bytes copied", n)
		checksum := hash.Sum(nil)
		fmt.Printf("SHA-256:\n%x\n", checksum)
		return
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

	// Call iocopy.CopyBuffer again to resume the calculation.
	log.Printf("call iocopy.CopyBuffer() again to resume the calculation")

	n2, err := iocopy.CopyBuffer(context.Background(), hash, resp2.Body, buf)
	if err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			// Got error during IO copy.
			log.Printf("iocopy.CopyBuffer() error: %v", err)
			return
		}
		// Stopped.
		log.Printf("iocopy.CopyBuffer() stopped, cause: %v\nbytes copied: %v", ctx.Err(), n)
	} else {
		log.Printf("iocopy.CopyBuffer() OK, %v bytes copied", n2)
	}

	log.Printf("total: %v bytes copied", n+n2)

	checksum := hash.Sum(nil)
	fmt.Printf("SHA-256:\n%x\n", checksum)

	// Output:
	// SHA-256:
	// dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
}

func ExampleCopyWithProgress() {
	// This example uses iocopy.CopyWithProgress to read stream from a remote file,
	// and compute its SHA-256 checksum.
	// It uses a timeout context to emulate user cancelation to stop the calculation.
	// Then it calls iocopy.CopyWithProgress again to resume the calculation.

	// SHA-256: dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
	url := "https://golang.google.cn/dl/go1.23.1.darwin-amd64.pkg"

	// Get response body(io.Reader) of the remote file.
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("http.Get() error: %v, url: %v", err, url)
		return
	}
	defer resp.Body.Close()

	// Get content length.
	total := int64(0)
	str := resp.Header.Get("Content-Length")
	if str != "" {
		if total, err = strconv.ParseInt(str, 10, 64); err != nil {
			log.Printf("parseInt() error: %v", err)
			return
		}
		log.Printf("Content-Length: %v", total)
	} else {
		log.Printf("failed to get Content-Length")
		return
	}

	// Create a hash.Hash(io.Writer) for SHA-256.
	// hash.Hash is an io.Writer.
	hash := sha256.New()

	// Use a timeout to emulate user's cancelation.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()

	// Call iocopy.CopyWithProgress to start the calculation.
	n, err := iocopy.CopyWithProgress(
		ctx,
		hash,
		resp.Body,
		total,
		0,
		func(total, prev, current int64, percent float32) {
			log.Printf("%v/%v(%.2f%%) bytes copied", prev+current, total, percent)
		})
	if err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			// Got error during IO copy.
			log.Printf("iocopy.CopyWithProgress() error: %v", err)
			return
		}
		// Stopped.
		log.Printf("iocopy.CopyWithProgress() stopped, cause: %v\nbytes copied: %v", ctx.Err(), n)
	} else {
		// IO copy done after first call, no need to resume.
		log.Printf("iocopy.CopyWithProgress() OK, %v bytes copied", n)
		checksum := hash.Sum(nil)
		fmt.Printf("SHA-256:\n%x\n", checksum)
		return
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

	// Call iocopy.CopyWithProgress again to resume the calculation.
	log.Printf("call iocopy.CopyWithProgress() again to resume the calculation")
	n2, err := iocopy.CopyWithProgress(
		context.Background(),
		hash,
		resp2.Body,
		total,
		n,
		func(total, prev, current int64, percent float32) {
			log.Printf("%v/%v(%.2f%%) bytes copied", prev+current, total, percent)
		},
	)
	if err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			// Got error during IO copy.
			log.Printf("iocopy.CopyWithProgress() error: %v", err)
			return
		}
		// Stopped.
		log.Printf("iocopy.CopyWithProgress() stopped, cause: %v\nbytes copied: %v", ctx.Err(), n)
	} else {
		log.Printf("iocopy.CopyWithProgress() OK, %v bytes copied", n2)
	}

	log.Printf("total: %v bytes copied", n+n2)

	checksum := hash.Sum(nil)
	fmt.Printf("SHA-256:\n%x\n", checksum)

	// Output:
	// SHA-256:
	// dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
}

func ExampleCopyBufferWithProgress() {
	// This example uses iocopy.CopyBufferWithProgress to read stream from a remote file,
	// and compute its SHA-256 checksum.
	// It uses a timeout context to emulate user cancelation to stop the calculation.
	// Then it calls iocopy.CopyBufferWithProgress again to resume the calculation.

	// SHA-256: dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
	url := "https://golang.google.cn/dl/go1.23.1.darwin-amd64.pkg"

	// Get response body(io.Reader) of the remote file.
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("http.Get() error: %v, url: %v", err, url)
		return
	}
	defer resp.Body.Close()

	// Get content length.
	total := int64(0)
	str := resp.Header.Get("Content-Length")
	if str != "" {
		if total, err = strconv.ParseInt(str, 10, 64); err != nil {
			log.Printf("parseInt() error: %v", err)
			return
		}
		log.Printf("Content-Length: %v", total)
	} else {
		log.Printf("failed to get Content-Length")
		return
	}

	// Create a hash.Hash(io.Writer) for SHA-256.
	// hash.Hash is an io.Writer.
	hash := sha256.New()

	// Use a timeout to emulate user's cancelation.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()

	// Create a buffer.
	buf := make([]byte, 1024*640)

	// Call iocopy.CopyBufferWithProgress to start the calculation.
	n, err := iocopy.CopyBufferWithProgress(
		ctx,
		hash,
		resp.Body,
		buf,
		total,
		0,
		func(total, prev, current int64, percent float32) {
			log.Printf("%v/%v(%.2f%%) bytes copied", prev+current, total, percent)
		})
	if err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			// Got error during IO copy.
			log.Printf("iocopy.CopyBufferWithProgress() error: %v", err)
			return
		}
		// Stopped.
		log.Printf("iocopy.CopyBufferWithProgress() stopped, cause: %v\nbytes copied: %v", ctx.Err(), n)
	} else {
		// IO copy done after first call, no need to resume.
		log.Printf("iocopy.CopyBufferWithProgress() OK, %v bytes copied", n)
		checksum := hash.Sum(nil)
		fmt.Printf("SHA-256:\n%x\n", checksum)
		return
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

	// Call iocopy.CopyBufferWithProgress again to resume the calculation.
	log.Printf("call iocopy.CopyBufferWithProgress() again to resume the calculation")
	n2, err := iocopy.CopyBufferWithProgress(
		context.Background(),
		hash,
		resp2.Body,
		buf,
		total,
		n,
		func(total, prev, current int64, percent float32) {
			log.Printf("%v/%v(%.2f%%) bytes copied", prev+current, total, percent)
		},
	)
	if err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			// Got error during IO copy.
			log.Printf("iocopy.CopyBufferWithProgress() error: %v", err)
			return
		}
		// Stopped.
		log.Printf("iocopy.CopyBufferWithProgress() stopped, cause: %v\nbytes copied: %v", ctx.Err(), n)
	} else {
		log.Printf("iocopy.CopyBufferWithProgress() OK, %v bytes copied", n2)
	}

	log.Printf("total: %v bytes copied", n+n2)

	checksum := hash.Sum(nil)
	fmt.Printf("SHA-256:\n%x\n", checksum)

	// Output:
	// SHA-256:
	// dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
}
