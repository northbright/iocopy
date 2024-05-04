package iocopy_test

import (
	"context"
	"log"

	"github.com/northbright/iocopy"
)

func ExampleNewCopyFileTask() {
	t, err := iocopy.NewCopyFileTask("/tmp/01.mts", "/Users/xxu/tmp/mts/01.mts")
	if err != nil {
		log.Printf("NewCopyFileTask error: %v", err)
		return
	}

	ctx := context.Background()
	//ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	//defer cancel()

	bufSize := uint(64 * 1024)
	iocopy.Do(
		ctx,
		t,
		bufSize,
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on written: %d/%d(%.2f%%)", copied, total, percent)
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, data []byte) {
			log.Printf("on stop(%v): %d/%d(%.2f%%), data: %s", cause, copied, total, percent, string(data))
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on ok: %d/%d(%.2f%%)", copied, total, percent)
		},
		func(err error) {
			log.Printf("on error: %v", err)
		},
	)

	// Output:
}
