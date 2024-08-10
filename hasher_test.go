package iocopy_test

import (
	"context"
	"log"
	"strings"

	"github.com/northbright/iocopy"
)

func ExampleNewHashTask() {
	str := "Hello, World!"
	r := strings.NewReader(str)
	algs := []string{"MD5", "SHA-1", "SHA-256"}

	t, err := iocopy.NewHashTask(algs, r)
	if err != nil {
		log.Printf("NewHashTask() error: %v", err)
		return
	}

	ctx := context.Background()
	bufSize := uint(32 * 1024)

	iocopy.Do(
		// Context
		ctx,
		// Task
		t,
		// Buffer size
		bufSize,
		// On bytes written
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			log.Printf("on written: %d/%d(%.2f%%)", copied, total, percent)
		},
		// On stop
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, state []byte) {
			log.Printf("on stop(%v): %d/%d(%.2f%%)\nstate: %s", cause, copied, total, percent, string(state))
		},
		// On ok
		func(isTotalKnown bool, total, copied, written uint64, percent float32, result []byte) {
			log.Printf("on ok: %d/%d(%.2f%%)\nresult: %s", copied, total, percent, string(result))

		},
		// On error
		func(err error) {
			log.Printf("on error: %v", err)
		},
	)

	// Output:
}
