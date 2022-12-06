# iocopy

iocopy is a [Golang](https://golang.org) package which provides functions to do IO copy in a new goroutine and returns an event channel for the caller.

## Usage
See [Example Code](https://github.com/northbright/iocopy/blob/master/iocopy_test.go)

```
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
	// go1.19.3.darwin-amd64.pkg
	// SHA-256:
	// a4941f5b09c43adeed13aaf435003a1e8852977037b3e6628d11047b087c4c66
	goPkgURL := "https://golang.google.cn/dl/go1.19.3.darwin-amd64.pkg"

	ctx := context.Background()

	resp, err := http.Get(goPkgURL)
	if err != nil {
		log.Printf("http.Get() error: %v", err)
		return
	}
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		log.Printf("status code is not 200 or 206")
		return
	}
	defer resp.Body.Close()

	contentLength := resp.Header.Get("Content-Length")
	total, _ := strconv.ParseInt(contentLength, 10, 64)
	if total <= 0 {
		log.Printf("Content-Length <= 0: %v", total)
		return
	}

	hash := sha256.New()
	ch := iocopy.Start(ctx, hash, resp.Body, 16*1024*1024, 500*time.Millisecond)

	for event := range ch {
		switch ev := event.(type) {
		case *iocopy.EventWritten:
			n := ev.Written()
			percent := float32(float64(n) / (float64(total) / float64(100)))
			log.Printf("on EventWritten: %v/%v bytes written(%.2f%%)", n, total, percent)

		case *iocopy.EventError:
			log.Printf("on EventError: %v", ev.Err())
			return
		case *iocopy.EventOK:
			n := ev.Written()
			percent := float32(float64(n) / (float64(total) / float64(100)))

			log.Printf("on EventOK: %v/%v bytes written(%.2f%%)", n, total, percent)
			checksum := hash.Sum(nil)
			fmt.Printf("SHA-256:\n%x", checksum)
			return
		}
	}

	// Output:
	// SHA-256:
	// a4941f5b09c43adeed13aaf435003a1e8852977037b3e6628d11047b087c4c66
}
```


## Docs
* <https://pkg.go.dev/github.com/northbright/iocopy>

## License
* [MIT License](LICENSE)
