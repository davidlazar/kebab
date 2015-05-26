package bucket

import (
	"os"

	"github.com/davidlazar/kebab/s3"
)

type Bucket interface {
	Abs(key string) string
	Put(key string, data []byte) error
	Get(key string) ([]byte, error)
	List() (keys, children []string, err error)
	Descend(child string) (Bucket, error)
	Destroy() error
}

func IsNotExist(err error) bool {
	switch e := err.(type) {
	case *s3.ServiceError:
		return e.Code == "NoSuchKey"
	default:
		return os.IsNotExist(err)
	}
}
