package iocopy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"

	"github.com/northbright/pathelper"
)

type CopyFileTask struct {
	Dst    string   `json:"dst"`
	Src    string   `json:"src"`
	Size   uint64   `json:"size,string"`
	Copied uint64   `json:"copied,string"`
	fw     *os.File `json:"-"`
	fr     *os.File `json:"-"`
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
	return t.fw
}

func (t *CopyFileTask) reader() io.Reader {
	return t.fr
}

func (t *CopyFileTask) MarshalJSON() ([]byte, error) {
	// Use a local type(alias) to avoid infinite loop when call json.Marshal() in MarshalJSON().
	type localCopyFileTask CopyFileTask

	a := (*localCopyFileTask)(t)
	return json.Marshal(a)
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
		fw:     fw,
		fr:     fr,
	}

	return t, nil
}

func LoadCopyFileTask(data []byte) (Task, error) {
	var err error

	t := &CopyFileTask{}

	if err = json.Unmarshal(data, t); err != nil {
		return nil, err
	}

	dir := path.Dir(t.Dst)
	if err = pathelper.CreateDirIfNotExists(dir, 0755); err != nil {
		return nil, err
	}

	t.fw, err = os.OpenFile(t.Dst, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	t.fr, err = os.Open(t.Src)
	if err != nil {
		return nil, err
	}

	if t.Copied != 0 {
		if _, err = t.fw.Seek(int64(t.Copied), 0); err != nil {
			return nil, err
		}

		if _, err = t.fr.Seek(int64(t.Copied), 0); err != nil {
			return nil, err
		}
	}

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
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, data []byte) {
			err = cause
		},
		func(isTotalKnown bool, total, copied, written uint64, percent float32) {
			err = nil
		},
		func(e error) {
			err = e
		},
	)
	return err
}
