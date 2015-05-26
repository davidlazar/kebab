package bucket

import (
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/nacl/secretbox"

	"github.com/davidlazar/go-crypto/secretkey"
)

var ErrAuth = errors.New("integrity check failure")

const BoxOverhead = 24 + secretbox.Overhead // nonce+mac

type encryptedBucket struct {
	bucket     Bucket
	privateKey *secretkey.Key
}

func NewEncryptedBucket(bucket Bucket, key *secretkey.Key) Bucket {
	return &encryptedBucket{
		bucket:     bucket,
		privateKey: key,
	}
}

func (b *encryptedBucket) Abs(key string) string {
	return b.bucket.Abs(key)
}

func (b *encryptedBucket) Put(key string, data []byte) error {
	box := sealBox(data, b.privateKey)
	return b.bucket.Put(key, box)
}

func (b *encryptedBucket) Get(key string) ([]byte, error) {
	box, err := b.bucket.Get(key)
	if err != nil {
		return nil, err
	}
	if len(box) < BoxOverhead {
		return nil, fmt.Errorf("short box")
	}

	data, ok := openBox(box, b.privateKey)
	if !ok {
		return nil, ErrAuth
	}

	return data, nil
}

func (b *encryptedBucket) List() (keys, children []string, err error) {
	return b.bucket.List()
}

func (b *encryptedBucket) Descend(child string) (Bucket, error) {
	bb, err := b.bucket.Descend(child)
	if err != nil {
		return nil, err
	}
	return &encryptedBucket{
		bucket:     bb,
		privateKey: b.privateKey,
	}, nil
}

func (b *encryptedBucket) Destroy() error {
	return b.bucket.Destroy()
}

func openBox(box []byte, key *secretkey.Key) ([]byte, bool) {
	var nonce [24]byte
	copy(nonce[:], box[0:24])
	return secretbox.Open(nil, box[24:], &nonce, (*[32]byte)(key))
}

func sealBox(data []byte, key *secretkey.Key) []byte {
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		panic("rand.Read error: " + err.Error())
	}
	return secretbox.Seal(nonce[:], data, &nonce, (*[32]byte)(key))
}
