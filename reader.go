package kebab

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
)

type Reader struct {
	bucket Bucket

	buf []byte
	n   int

	boxes []boxhash
	bn    int

	total int64
}

func NewReader(bucket Bucket) (*Reader, error) {
	r := &Reader{
		bucket: bucket,
	}

	metajson, err := r.bucket.Get("meta")
	if err != nil {
		return nil, fmt.Errorf("Get(%q): %s", "meta", err)
	}
	var meta Metadata
	if err := json.Unmarshal(metajson, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %s", err)
	}
	r.boxes = meta.Boxes
	return r, nil
}

func (r *Reader) Read(p []byte) (n int, err error) {
	defer func() { r.total += int64(n) }()

	if r.n == len(r.buf) {
		if err = r.fill(); err != nil {
			return 0, err
		}
	}
	n = copy(p, r.buf[r.n:])
	r.n += n
	return n, nil
}

func (r *Reader) fill() (err error) {
	if r.bn == len(r.boxes) {
		return io.EOF
	}
	key := fmt.Sprintf("%05d", r.bn)
	if r.buf, err = r.bucket.Get(key); err != nil {
		return err
	}
	h := sha256.Sum256(r.buf)
	if !bytes.Equal(h[:], r.boxes[r.bn][:]) {
		return fmt.Errorf("Get(%q): hash mismatch", key)
	}
	r.n = 0
	r.bn += 1
	return nil
}

func (r *Reader) Size() int64 {
	return r.total
}
