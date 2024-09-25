package progress_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/northbright/iocopy"
	"github.com/northbright/iocopy/progress"
)

func ExampleNew() {
	// This example uses iocopy.Copy to read stream from a remote file,
	// and compute its SHA-256 checksum.

	// SHA-256: dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
	url := "https://golang.google.cn/dl/go1.23.1.darwin-amd64.pkg"

	// Get response body(io.Reader) of the remote file.
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("http.Get() error: %v, url: %v", err, url)
	}
	defer resp.Body.Close()

	// Get size of the remote file.
	size := int64(0)
	str := resp.Header.Get("Content-Length")
	if str != "" {
		size, _ = strconv.ParseInt(str, 10, 64)
	}

	// Create a hash.Hash(io.Writer) for SHA-256.
	// hash.Hash is an io.Writer.
	hash := sha256.New()

	// Create a Progress which implements io.Writer interface.
	p := progress.New(
		// Total size.
		size,
		// OnWrittenFunc callback
		progress.OnWritten(func(total, prev, current int64, percent float32) {
			log.Printf("%v / %v(%.2f%%) bytes read and computed", current, total, percent)
		}),
	)

	// Create a multiple writer and duplicate the writes to p.
	mw := io.MultiWriter(hash, p)

	// Create a context.
	// Uncomment below lines to test stopping IO copy with a timeout.
	//ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*800)
	//defer cancel()
	ctx := context.Background()

	// Create a channel used to make the progress goroutine to receive the signal to exit.
	chExit := make(chan struct{}, 1)
	defer func() {
		chExit <- struct{}{}
		close(chExit)
	}()

	// Starts a new goroutine and a ticker to call the callback to report progress.
	// The goroutine exits when an empty struct is send to chExit or ctx.Done().
	p.Start(ctx, chExit)

	// Call iocopy.Copy to read stream of the remote file and compute SHA-256 checksum.
	// It blocks the main goroutine.
	n, err := iocopy.Copy(ctx, mw, resp.Body)
	if err != nil {
		if err != context.Canceled && err != context.DeadlineExceeded {
			log.Printf("iocopy.Copy() error: %v", err)
			return
		}

		log.Printf("iocopy.Copy() stopped, cause: %v", ctx.Err())
		return
	}

	log.Printf("iocopy.Copy() OK. %v bytes copied", n)

	// Get checksum.
	checksum := hash.Sum(nil)
	fmt.Printf("SHA-256:\n%x\n", checksum)

	// Output:
	// SHA-256:
	// dd9e772686ed908bcff94b6144322d4e2473a7dcd7c696b7e8b6d12f23c887fd
}
