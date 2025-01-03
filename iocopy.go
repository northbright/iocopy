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

// writeFunc is used to implement [io.Writer] interface and capture the [context.Context] parameter.
type writeFunc func(p []byte) (n int, err error)

// Write implements [io.Writer] interface.
func (wf writeFunc) Write(p []byte) (n int, err error) {
	return wf(p)
}

// OnWrittenFunc is the callback function when bytes are copied successfully.
// total: total number of bytes to copy.
// A negative value indicates total size is unknown and percent should be ignored(always 0).
// prev: number of bytes copied previously.
// current: number of bytes copied in current copy.
// percent: percent copied.
type OnWrittenFunc func(total, prev, current int64, percent float32)

// computePercent returns the percentage.
// total: total number of the bytes to copy.
// A negative value indicates total size is unknown and it returns 0 as percent.
// prev: the number of the bytes copied previously.
// current: the number of bytes written currently.
func computePercent(total, prev, current int64) float32 {
	if total == 0 {
		return 100
	}

	if total < 0 {
		return 0
	}

	if prev+current < 0 {
		return 0
	}

	return float32(float64(prev+current) / (float64(total) / float64(100)))
}

// CopyWithProgress wraps [io.Copy]. It accepts [context.Context] to make IO copy cancalable.
// It also accepts callback function on bytes written to report progress.
// total: total number of bytes to copy.
// prev: number of bytes copied previously.
// It can be used to resume the IO copy.
// 1. Set prev to 0 when call CopyWithProgress for the first time.
// 2. User stops the IO copy and CopyWithProgress returns the number of bytes written and error.
// 3. Check if err == context.Canceled || err == context.DeadlineExceeded.
// 4. Set prev to the "written" return value of previous CopyWithProgress when make next call to resume the IO copy.
// fn: callback on bytes written.
func CopyWithProgress(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	total int64,
	prev int64,
	fn OnWrittenFunc) (written int64, err error) {

	var (
		current    int64
		percent    float32
		oldPercent float32
	)

	return io.Copy(
		writeFunc(func(p []byte) (n int, err error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default:
				n, err = dst.Write(p)
				if err != nil {
					return n, err
				}

				if fn != nil {
					current += int64(n)
					if total == prev+current {
						fn(total, prev, current, 100)
					} else {
						percent = computePercent(total, prev, current)
						if percent != oldPercent {
							fn(total, prev, current, percent)
							oldPercent = percent
						}
					}
				}
				return n, nil
			}
		}),
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

// CopyBufferWithProgress wraps [io.CopyBuffer]. It accepts [context.Context] to make IO copy cancalable.
// It also accepts callback function on bytes written to report progress.
// total: total number of bytes to copy.
// prev: number of bytes copied previously.
// It can be used to resume the IO copy.
// 1. Set prev to 0 when call CopyBufferWithProgress for the first time.
// 2. User stops the IO copy and CopyBufferWithProgress returns the number of bytes written and error.
// 3. Check if err == context.Canceled || err == context.DeadlineExceeded.
// 4. Set prev to the "written" return value of previous CopyBufferWithProgress when make next call to resume the IO copy.
// fn: callback on bytes written.
func CopyBufferWithProgress(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	buf []byte,
	total int64,
	prev int64,
	fn OnWrittenFunc) (written int64, err error) {

	var (
		current    int64
		percent    float32
		oldPercent float32
	)

	return io.CopyBuffer(
		writeFunc(func(p []byte) (n int, err error) {
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			default:
				n, err = dst.Write(p)
				if err != nil {
					return n, err
				}

				if fn != nil {
					current += int64(n)
					if total == prev+current {
						fn(total, prev, current, 100)
					} else {
						percent = computePercent(total, prev, current)
						if percent != oldPercent {
							fn(total, prev, current, percent)
							oldPercent = percent
						}
					}
				}
				return n, nil
			}
		}),
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

// Copy wraps [io.Copy]. It accepts [context.Context] to make IO copy cancalable.
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

// CopyBuffer wraps [io.CopyBuffer]. It accepts [context.Context] to make IO copy cancalable.
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
