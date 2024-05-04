package iocopy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/northbright/httputil"
	"github.com/northbright/pathelper"
)

type DownloadTask struct {
	Dst              string    `json:"dst"`
	Url              string    `json:"url"`
	IsSizeKnown      bool      `json:"is_size_known"`
	Size             uint64    `json:"size"`
	IsRangeSupported bool      `json:"is_range_supported"`
	Downloaded       uint64    `json:"downloaded",string`
	w                io.Writer `json:"-"`
	r                io.Reader `json:"-"`
}

func (t *DownloadTask) total() (bool, uint64) {
	return t.IsSizeKnown, t.Size
}

func (t *DownloadTask) copied() uint64 {
	return t.Downloaded
}

func (t *DownloadTask) setCopied(copied uint64) {
	t.Downloaded = copied
}

func (t *DownloadTask) writer() io.Writer {
	return t.w
}

func (t *DownloadTask) reader() io.Reader {
	return t.r
}

func (t *DownloadTask) MarshalJSON() ([]byte, error) {
	// Use a local type(alias) to avoid infinite loop when call json.Marshal() in MarshalJSON().
	type localDownloadTask DownloadTask

	a := (*localDownloadTask)(t)
	return json.Marshal(a)
}

func (t *DownloadTask) UnmarshalJSON(data []byte) error {
	var (
		resp *http.Response
	)

	// Use a local type(alias) to avoid infinite loop when call json.Marshal() in MarshalJSON().
	type localDownloadTask DownloadTask
	a := (*localDownloadTask)(t)

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

	// Check if it can resume downloading.
	if !t.IsRangeSupported {
		resp, t.IsSizeKnown, t.Size, t.IsRangeSupported, err = httputil.GetResp(t.Url)
		if err != nil {
			return err
		}

		// Reset number of bytes downloaded to 0.
		t.Downloaded = 0
	} else {
		resp, _, err = httputil.GetRespOfRangeStart(t.Url, t.Downloaded)
		if err != nil {
			return err
		}

		if _, err = w.Seek(int64(t.Downloaded), 0); err != nil {
			return err
		}
	}

	t.w = w
	t.r = resp.Body

	return nil
}

func NewDownloadTask(Dst, Url string) (Task, error) {
	resp, isSizeKnown, size, isRangeSupported, err := httputil.GetResp(Url)
	if err != nil {
		return nil, err
	}

	dir := path.Dir(Dst)
	if err := pathelper.CreateDirIfNotExists(dir, 0755); err != nil {
		return nil, err
	}

	w, err := os.Create(Dst)
	if err != nil {
		return nil, err
	}

	t := &DownloadTask{
		Dst:              Dst,
		Url:              Url,
		IsSizeKnown:      isSizeKnown,
		Size:             size,
		IsRangeSupported: isRangeSupported,
		Downloaded:       0,
		w:                w,
		r:                resp.Body,
	}

	return t, nil
}

func LoadDownloadTask(data []byte) (Task, error) {
	t := &DownloadTask{}

	if err := t.UnmarshalJSON(data); err != nil {
		return nil, err
	}

	return t, nil
}

func Download(ctx context.Context, Dst, Url string, bufSize uint) error {
	var (
		err = fmt.Errorf("unexpected behavior")
	)
	t, err := NewDownloadTask(Dst, Url)
	if err != nil {
		log.Printf("NewDownloadTask() error: %v", err)
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
