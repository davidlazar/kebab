package bucket

import (
	"fmt"
	"strings"

	"github.com/davidlazar/kebab/s3"
)

type s3Bucket struct {
	bucket *s3.Bucket
	prefix string
}

func NewS3Bucket(b *s3.Bucket) Bucket {
	return &s3Bucket{bucket: b}
}

func NewS3BucketFromFile(path string) (Bucket, error) {
	s3b, err := s3.NewBucketFromFile(path)
	if err != nil {
		return nil, err
	}
	return NewS3Bucket(s3b), nil
}

func (b *s3Bucket) Abs(key string) string {
	return b.prefix + key
}

func (b *s3Bucket) Put(key string, data []byte) error {
	return b.bucket.Put(b.Abs(key), data)
}

func (b *s3Bucket) Get(key string) ([]byte, error) {
	return b.bucket.Get(b.Abs(key))
}

func (b *s3Bucket) List() (keys []string, children []string, err error) {
	list, err := b.bucket.List(b.prefix, "/")
	if err != nil {
		return nil, nil, fmt.Errorf("List(%q, %q): %s", b.prefix, "/", err)
	}
	if list.IsTruncated {
		err = fmt.Errorf("List(%q, %q): results truncated", b.prefix, "/")
	}

	for _, k := range list.Contents {
		keys = append(keys, strings.TrimPrefix(k, b.prefix))
	}
	for _, d := range list.CommonPrefixes {
		children = append(children, strings.TrimPrefix(strings.TrimSuffix(d, "/"), b.prefix))
	}

	return
}

func (b *s3Bucket) Descend(child string) (Bucket, error) {
	if !strings.HasSuffix(child, "/") {
		child += "/"
	}
	return &s3Bucket{
		bucket: b.bucket,
		prefix: b.prefix + child,
	}, nil
}

func (b *s3Bucket) Destroy() error {
	for {
		list, err := b.bucket.List(b.prefix, "")
		if err != nil {
			return fmt.Errorf("List(%q, %q): %s", b.prefix, "", err)
		}
		if len(list.Contents) == 0 {
			return nil
		}

		del, err := b.bucket.Delete(list.Contents)
		if err != nil {
			return fmt.Errorf("Delete(%q, ...): %s", list.Contents[0], err)
		}
		if err = del.GetError(); err != nil {
			return err
		}
	}
}
