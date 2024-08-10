package iocopy

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"sort"
)

var (
	// Not encoding.BinaryMarshaler
	ErrNotBinaryMarshaler = errors.New("not binary marshaler")

	// Not encoding.BinaryUnmarshaler
	ErrNotBinaryUnmarshaler = errors.New("not binary unmarshaler")
)

type HashTask struct {
	Algs     []string             `json:"algs"`
	Computed uint64               `json:"computed,string"`
	States   map[string][]byte    `json:"states"`
	r        io.Reader            `json:"-"`
	hashes   map[string]hash.Hash `json:"-"`
}

type FileHashTask struct {
	Src  string `json:"src"`
	Size uint64 `json:"size"`
	HashTask
}

func (t *HashTask) total() (bool, uint64) {
	return false, 0
}

func (t *HashTask) copied() uint64 {
	return t.Computed
}

func (t *HashTask) setCopied(copied uint64) {
	t.Computed = copied
}

func (t *HashTask) writer() io.Writer {
	var writers []io.Writer

	for _, alg := range t.Algs {
		writers = append(writers, t.hashes[alg])
	}

	return io.MultiWriter(writers...)
}

func (t *HashTask) reader() io.Reader {
	return t.r
}

func (t *HashTask) state() ([]byte, error) {
	for alg, h := range t.hashes {
		marshaler, ok := h.(encoding.BinaryMarshaler)
		if !ok {
			return nil, ErrNotBinaryMarshaler
		}

		state, err := marshaler.MarshalBinary()
		if err != nil {
			return nil, err
		}

		t.States[alg] = state
	}

	return json.MarshalIndent(t, "", "    ")
}

func (t *HashTask) result() ([]byte, error) {
	type Result struct {
		Checksums map[string]string `json:"checksums"`
	}

	r := Result{Checksums: make(map[string]string)}

	for alg, h := range t.hashes {
		r.Checksums[alg] = fmt.Sprintf("%X", h.Sum(nil))
	}

	return json.MarshalIndent(r, "", "    ")
}

func (t *HashTask) Checksums() map[string][]byte {
	var checksums = make(map[string][]byte)

	for alg, hash := range t.hashes {
		checksums[alg] = hash.Sum(nil)
	}

	return checksums
}

// crc32NewIEEE is a wrapper of crc32.NewIEEE.
// Make it possible to return a hash.Hash instead of hash.Hash32.
func crc32NewIEEE() hash.Hash {
	return hash.Hash(crc32.NewIEEE())
}

var (
	hashAlgsToNewFuncs = map[string]func() hash.Hash{
		"MD5":     md5.New,
		"SHA-1":   sha1.New,
		"SHA-256": sha256.New,
		"SHA-512": sha512.New,
		"CRC-32":  crc32NewIEEE,
	}

	// ErrUnSupportedHashAlg indicates that the hash algorithm is not supported.
	ErrUnSupportedHashAlg = errors.New("unsupported hash algorithm")
)

// SupportedHashAlgs returns supported hash algorithms of this package.
func SupportedHashAlgs() []string {
	var algs []string

	for alg := range hashAlgsToNewFuncs {
		algs = append(algs, alg)
	}

	// Sort hash algorithms by names.
	sort.Slice(algs, func(i, j int) bool {
		return algs[i] < algs[j]
	})

	return algs
}

func NewHashTask(algs []string, r io.Reader) (Task, error) {
	if algs == nil {
		// Use all supported hash algorithms by default.
		algs = SupportedHashAlgs()
	}

	hashes := make(map[string]hash.Hash)

	for _, alg := range algs {
		f, ok := hashAlgsToNewFuncs[alg]
		if !ok {
			return nil, ErrUnSupportedHashAlg
		}
		// Call f function to new a hash.Hash and insert it to the map.
		hashes[alg] = f()
	}

	t := &HashTask{
		Algs:     algs,
		Computed: 0,
		States:   make(map[string][]byte),
		r:        r,
		hashes:   hashes,
	}

	return t, nil
}

/*
func LoadHashTask(state []byte) (Task, error) {
	var err error

	t := &HashTask{}

	if err = json.Unmarshal(state, t); err != nil {
		return nil, err
	}

	var writers []io.Writer

	// Load binary state for each hash.Hash.
	for alg, hash := range t.hashes {
		unmarshaler, ok := hash.(encoding.BinaryUnmarshaler)
		if !ok {
			return nil, ErrNotBinaryUnmarshaler
		}

		if err := unmarshaler.UnmarshalBinary(t.States[alg]); err != nil {
			return nil, err
		}

		writers = append(writers, hash)
	}

	w := io.MultiWriter(writers...)

	fr, err := os.Open(t.Src)
	if err != nil {
		return nil, err
	}

	if t.Computed != 0 {
		if _, err = fr.Seek(int64(t.Computed), 0); err != nil {
			return nil, err
		}
	}

	t.w = w
	t.r = fr

	return t, nil
}
*/
