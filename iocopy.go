package iocopy

import (
	"context"
	"io"
)

// readFunc is used to implement [io.Reader] interface and capture the [context.Context] parameter.
type readFunc func(p []byte) (n int, err error)

// Read implements [io.Reader] interface.
func (rf readFunc) Read(p []byte) (n int, err error) {
	return rf(p)
}

// Copy wraps [io.Copy] and accepts [context.Context] parameter.
func Copy(ctx context.Context, dst io.Writer, src io.Reader) (written int64, err error) {
	return io.Copy(
		dst,
		readFunc(func(p []byte) (n int, err error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default:
				return src.Read(p)
			}
		}),
	)
}

// CopyBuffer wraps [io.CopyBuffer] and accepts [context.Context] parameter.
func CopyBuffer(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	return io.CopyBuffer(
		dst,
		readFunc(func(p []byte) (n int, err error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default:
				return src.Read(p)
			}
		}),
		buf,
	)
}

// OnWrittenFunc is the callback function when bytes are copied successfully.
// total: total number of bytes to copy.
// prev: number of bytes copied previously.
// current: number of bytes copied in current copy.
// percent: percent copied.
type OnWrittenFunc func(total, prev, current int64, percent float32)
