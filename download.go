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
	Dst              string         `json:"dst"`
	Url              string         `json:"url"`
	IsSizeKnown      bool           `json:"is_size_known"`
	Size             uint64         `json:"size,string"`
	IsRangeSupported bool           `json:"is_range_supported"`
	Downloaded       uint64         `json:"downloaded,string"`
	fw               *os.File       `json:"-"`
	resp             *http.Response `json:"-"`
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
	return t.fw
}

func (t *DownloadTask) reader() io.Reader {
	return t.resp.Body
}

func (t *DownloadTask) state() ([]byte, error) {
	return json.Marshal(t)
}

func NewDownloadTask(dst, url string) (Task, error) {
	resp, isSizeKnown, size, isRangeSupported, err := httputil.GetResp(url)
	if err != nil {
		return nil, err
	}

	dir := path.Dir(dst)
	if err := pathelper.CreateDirIfNotExists(dir, 0755); err != nil {
		return nil, err
	}

	fw, err := os.Create(dst)
	if err != nil {
		return nil, err
	}

	t := &DownloadTask{
		Dst:              dst,
		Url:              url,
		IsSizeKnown:      isSizeKnown,
		Size:             size,
		IsRangeSupported: isRangeSupported,
		Downloaded:       0,
		fw:               fw,
		resp:             resp,
	}

	return t, nil
}

func LoadDownloadTask(state []byte) (Task, error) {
	var err error

	t := &DownloadTask{}

	if err = json.Unmarshal(state, t); err != nil {
		return nil, err
	}

	dir := path.Dir(t.Dst)
	if err := pathelper.CreateDirIfNotExists(dir, 0755); err != nil {
		return nil, err
	}

	t.fw, err = os.OpenFile(t.Dst, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	// Check if it can resume downloading.
	if !t.IsRangeSupported {
		t.resp, t.IsSizeKnown, t.Size, t.IsRangeSupported, err = httputil.GetResp(t.Url)
		if err != nil {
			return nil, err
		}

		// Reset number of bytes downloaded to 0.
		t.Downloaded = 0
	} else {
		t.resp, _, err = httputil.GetRespOfRangeStart(t.Url, t.Downloaded)
		if err != nil {
			return nil, err
		}

		if _, err = t.fw.Seek(int64(t.Downloaded), 0); err != nil {
			return nil, err
		}
	}

	return t, nil
}

func Download(ctx context.Context, dst, url string, bufSize uint) error {
	var (
		err = fmt.Errorf("unexpected behavior")
	)
	t, err := NewDownloadTask(dst, url)
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
		func(isTotalKnown bool, total, copied, written uint64, percent float32, cause error, state []byte) {
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
