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
	Dst    string    `json:"dst"`
	Src    string    `json:"src"`
	Copied uint64    `json:"copied",string`
	w      io.Writer `json:"-"`
	r      io.Reader `json:"-"`
}

func (t *CopyFileTask) total() (bool, uint64) {
	fi, err := os.Lstat(t.Src)
	if err != nil {
		return false, 0
	}

	return true, uint64(fi.Size())
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

func (t *CopyFileTask) MarshalJSON() ([]byte, error) {
	// Use a local type(alias) to avoid infinite loop when call json.Marshal() in MarshalJSON().
	type localCopyFileTask CopyFileTask

	a := (*localCopyFileTask)(t)
	return json.Marshal(a)
}

func (t *CopyFileTask) UnmarshalJSON(data []byte) error {
	// Use a local type(alias) to avoid infinite loop when call json.Marshal() in MarshalJSON().
	type localCopyFileTask CopyFileTask

	a := (*localCopyFileTask)(t)

	if err := json.Unmarshal(data, a); err != nil {
		return err
	}

	dir := path.Dir(t.Dst)
	if err := pathelper.CreateDirIfNotExists(dir, 0755); err != nil {
		return err
	}

	w, err := os.OpenFile(t.Dst, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	r, err := os.Open(t.Src)
	if err != nil {
		return err
	}

	if t.Copied != 0 {
		if _, err = w.Seek(int64(t.Copied), 0); err != nil {
			return err
		}

		if _, err = r.Seek(int64(t.Copied), 0); err != nil {
			return err
		}
	}

	t.w = w
	t.r = r

	return nil
}

func NewCopyFileTask(Dst, Src string) (Task, error) {
	dir := path.Dir(Dst)
	if err := pathelper.CreateDirIfNotExists(dir, 0755); err != nil {
		return nil, err
	}

	w, err := os.Create(Dst)
	if err != nil {
		return nil, err
	}

	r, err := os.Open(Src)
	if err != nil {
		return nil, err
	}

	t := &CopyFileTask{
		Dst:    Dst,
		Src:    Src,
		Copied: 0,
		w:      w,
		r:      r,
	}

	return t, nil
}

func LoadCopyFileTask(data []byte) (Task, error) {
	t := &CopyFileTask{}

	if err := t.UnmarshalJSON(data); err != nil {
		return nil, err
	}

	return t, nil
}

func CopyFile(ctx context.Context, Dst, Src string, bufSize uint) error {
	var (
		err = fmt.Errorf("unexpected behavior")
	)
	t, err := NewCopyFileTask(Dst, Src)
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
