package kebab

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
)

const Version = 0

type boxhash [sha256.Size]byte

type Writer struct {
	err error

	bucket Bucket
	boxes  []boxhash

	buf []byte
	n   int

	total int64
}

func NewWriter(bucket Bucket, boxSize int) *Writer {
	w := &Writer{
		bucket: bucket,
		buf:    make([]byte, boxSize),
	}
	return w
}

func (w *Writer) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return n, w.err
	}
	defer func() { w.total += int64(n) }()
	for len(p) > len(w.buf)-w.n {
		c := copy(w.buf[w.n:], p)
		w.n += c
		n += c
		p = p[c:]
		if w.err = w.flush(); w.err != nil {
			return n, w.err
		}
	}
	c := copy(w.buf[w.n:], p)
	w.n += c
	n += c
	return n, nil
}

func (w *Writer) flush() error {
	if w.n == 0 {
		return nil
	}
	key := fmt.Sprintf("%05d", len(w.boxes))
	if err := w.bucket.Put(key, w.buf[0:w.n]); err != nil {
		return err
	}
	w.boxes = append(w.boxes, sha256.Sum256(w.buf[0:w.n]))
	w.n = 0
	return nil
}

type Metadata struct {
	Version int
	Boxes   []boxhash
}

func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}

	if w.err = w.flush(); w.err != nil {
		return w.err
	}

	m := Metadata{
		Version: Version,
		Boxes:   w.boxes,
	}
	metajson, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	if w.err = w.bucket.Put("meta", metajson); w.err != nil {
		return w.err
	}

	w.err = errors.New("already closed")
	return nil
}

func (w *Writer) Size() int64 {
	return w.total
}
