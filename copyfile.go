package iocopy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"

	"github.com/northbright/pathelper"
)

type CopyFileTask struct {
	Dst    string    `json:"dst"`
	Src    string    `json:"src"`
	Size   uint64    `json:"size,string"`
	Copied uint64    `json:"copied,string"`
	w      io.Writer `json:"-"`
	r      io.Reader `json:"-"`
}

func (t *CopyFileTask) total() (bool, uint64) {
	return true, t.Size
}

func (t *CopyFileTask) copied() uint64 {
	return t.Copied
}

func (t *CopyFileTask) setCopied(copied uint64) {
	t.Copied = copied
}

func (t *CopyFileTask) writer() io.Writer {
	return t.w
}

func (t *CopyFileTask) reader() io.Reader {
	return t.r
}

func (t *CopyFileTask) state() ([]byte, error) {
	return json.MarshalIndent(t, "", "    ")
}

func (t *CopyFileTask) result() ([]byte, error) {
	return json.MarshalIndent(t, "", "    ")
}

func NewCopyFileTask(dst, src string) (Task, error) {
	// Get src file info.
	fi, err := os.Lstat(src)
	if err != nil {
		return nil, err
	}

	// Check if src's a regular file.
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("src's not a regular file")
	}

	// Get the source file's size.
	size := uint64(fi.Size())

	// Make dest file's dir if it does not exist.
	dir := path.Dir(dst)
	if err := pathelper.CreateDirIfNotExists(dir, 0755); err != nil {
		return nil, err
	}

	fw, err := os.Create(dst)
	if err != nil {
		return nil, err
	}

	fr, err := os.Open(src)
	if err != nil {
		return nil, err
	}

	t := &CopyFileTask{
		Dst:    dst,
		Src:    src,
		Size:   size,
		Copied: 0,
		w:      fw,
		r:      fr,
	}

	return t, nil
}

func LoadCopyFileTask(state []byte) (Task, error) {
	var err error

	t := &CopyFileTask{}

	if err = json.Unmarshal(state, t); err != nil {
		return nil, err
	}

	dir := path.Dir(t.Dst)
	if err = pathelper.CreateDirIfNotExists(dir, 0755); err != nil {
		return nil, err
	}

	fw, err := os.OpenFile(t.Dst, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	fr, err := os.Open(t.Src)
	if err != nil {
		return nil, err
	}

	if t.Copied != 0 {
		if _, err = fw.Seek(int64(t.Copied), 0); err != nil {
			return nil, err
		}

		if _, err = fr.Seek(int64(t.Copied), 0); err != nil {
			return nil, err
		}
	}

	t.w = fw
	t.r = fr

	return t, nil
}

func CopyFile(ctx context.Context, dst, src string, bufSize uint) error {
	var (
		err = fmt.Errorf("unexpected behavior")
	)
	t, err := NewCopyFileTask(dst, src)
	if err != nil {
		log.Printf("NewCopyFileTask() error: %v", err)
		return err
	}

	if bufSize == 0 {
		bufSize = DefaultBufSize
	}

	Do(
		ctx,
		t,
		bufSize,
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, state []byte) {
			err = cause
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32, result []byte) {
			err = nil
		},
		func(e error) {
			err = e
		},
	)
	return err
}

// NewCopyFileFromFSTask creates a task for copying file from fs.FS.
// There's no function to load a task for resuming coping file from fs.FS because io.ReadAt or io.Seeker is optional for fs.File interface.
func NewCopyFileFromFSTask(dst string, srcFS fs.FS, src string) (Task, error) {
	// Open src file.
	fr, err := srcFS.Open(src)

	// Get the size of src file.
	fi, err := fr.Stat()
	if err != nil {
		return nil, err
	}

	// Check if src's a regular file.
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("not regular file")
	}

	// Get total size of src.
	size := uint64(fi.Size())

	// Make dest file's dir if it does not exist.
	dir := path.Dir(dst)
	if err := pathelper.CreateDirIfNotExists(dir, 0755); err != nil {
		return nil, err
	}

	fw, err := os.Create(dst)
	if err != nil {
		return nil, err
	}

	t := &CopyFileTask{
		Dst:    dst,
		Src:    src,
		Size:   size,
		Copied: 0,
		w:      fw,
		r:      fr,
	}

	return t, nil
}

func CopyFileFromFS(ctx context.Context, dst string, srcFS fs.FS, src string, bufSize uint) error {
	var (
		err = fmt.Errorf("unexpected behavior")
	)
	t, err := NewCopyFileFromFSTask(dst, srcFS, src)
	if err != nil {
		log.Printf("NewCopyFileFromFSTask() error: %v", err)
		return err
	}

	if bufSize == 0 {
		bufSize = DefaultBufSize
	}

	Do(
		ctx,
		t,
		bufSize,
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, state []byte) {
			err = cause
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32, result []byte) {
			err = nil
		},
		func(e error) {
			err = e
		},
	)
	return err
}
