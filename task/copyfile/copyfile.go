package copyfile

import (
	"encoding/json"
	"io"
	"os"
	"path"

	"github.com/northbright/pathelper"
)

type Task struct {
	Dst       string    `json:"dst"`
	Src       string    `json:"src"`
	NumCopied uint64    `json:"copied",string`
	w         io.Writer `json:"-"`
	r         io.Reader `json:"-"`
}

func (t *Task) Total() (bool, uint64) {
	fi, err := os.Lstat(t.Src)
	if err != nil {
		return false, 0
	}

	return true, uint64(fi.Size())
}

func (t *Task) Copied() uint64 {
	return t.NumCopied
}

func (t *Task) SetCopied(copied uint64) {
	t.NumCopied = copied
}

func (t *Task) Writer() io.Writer {
	return t.w
}

func (t *Task) Reader() io.Reader {
	return t.r
}

func (t *Task) MarshalJSON() ([]byte, error) {
	// Use a local type(alias) to avoid infinite loop when call json.Marshal() in MarshalJSON().
	type localTask Task

	a := (*localTask)(t)
	return json.Marshal(a)
}

func (t *Task) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, t); err != nil {
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

	if t.NumCopied != 0 {
		if _, err = w.Seek(int64(t.NumCopied), 0); err != nil {
			return err
		}

		if _, err = r.Seek(int64(t.NumCopied), 0); err != nil {
			return err
		}
	}

	t.w = w
	t.r = r

	return nil
}

func New(Dst, Src string) (*Task, error) {
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

	t := &Task{
		Dst:       Dst,
		Src:       Src,
		NumCopied: 0,
		w:         w,
		r:         r,
	}

	return t, nil
}

func Load(data []byte) (*Task, error) {
	t := &Task{}

	if err := t.UnmarshalJSON(data); err != nil {
		return nil, err
	}

	return t, nil
}
