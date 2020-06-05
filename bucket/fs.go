package bucket

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type fileBucket struct {
	root string
}

func NewFileBucket(path string) (Bucket, error) {
	// TODO check if path is a file
	return &fileBucket{root: path}, nil
}

func (b *fileBucket) Abs(key string) string {
	return filepath.Join(b.root, key)
}

func (b *fileBucket) Put(key string, data []byte) error {
	p := b.Abs(key)
	if err := os.MkdirAll(filepath.Dir(p), os.ModeDir|0700); err != nil {
		return err
	}
	return ioutil.WriteFile(p, data, 0600)
}

func (b *fileBucket) Get(key string) ([]byte, error) {
	return ioutil.ReadFile(b.Abs(key))
}

func (b *fileBucket) List() (keys, children []string, err error) {
	list, err := ioutil.ReadDir(b.root)
	if os.IsNotExist(err) {
		return nil, nil, nil
	} else if err != nil {
		return
	}

	for _, x := range list {
		if x.IsDir() {
			children = append(children, x.Name())
		} else {
			keys = append(keys, x.Name())
		}
	}

	return
}

func (b *fileBucket) Descend(child string) (Bucket, error) {
	return NewFileBucket(b.Abs(child))
}

func (b *fileBucket) Destroy() error {
	abs, err := filepath.Abs(b.root)
	if err != nil {
		return fmt.Errorf("filepath.Abs(%q): %s", b.root, err)
	}

	if filepath.Dir(abs) == "/" {
		return fmt.Errorf("refusing to destroy top-level directory %q", abs)
	}

	if err := os.RemoveAll(b.root); err != nil {
		return fmt.Errorf("os.RemoveAll(%q): %s", b.root, err)
	}

	return nil
}
