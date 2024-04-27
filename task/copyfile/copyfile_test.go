package copyfile_test

import (
	"context"
	"log"
	"time"

	"github.com/northbright/iocopy/task"
	"github.com/northbright/iocopy/task/copyfile"
)

func Example() {
	t, _ := copyfile.New("/tmp/01.mp4", "/Users/xxu/01.mp4")

	//ctx := context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()

	bufSize := uint(64 * 1024)
	task.Do(
		ctx,
		t,
		bufSize,
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on written: %d/%d(%.2f)", copied, total, percent)
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, data []byte) {
			log.Printf("on stop(%v): %d/%d(%.2f), data: %s", cause, copied, total, percent, string(data))
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on ok: %d/%d(%.2f)", copied, total, percent)
		},
		func(err error) {
			log.Printf("on error: %v", err)
		},
	)

	// Output:
}
