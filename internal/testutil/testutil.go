package testutil

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/davidlazar/go-crypto/secretkey"
	"github.com/davidlazar/kebab/bucket"
)

var s3conf = flag.String("s3", "", "s3 configuration")

var (
	TempDir   string
	TempS3    bucket.Bucket
	SkipS3    = true
	PromptLog *bucket.PromptLogger
)

func Setup() {
	flag.Parse()

	var err error
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("os.Getwd: %s", err)
	}
	TempDir, err = ioutil.TempDir(wd, "kebab_testing_")
	if err != nil {
		log.Fatalf("ioutil.TempDir failed: %s", err)
	}

	stdlog := log.New(os.Stderr, "RecoverableBucketLog: ", 0)
	PromptLog = &bucket.PromptLogger{Logger: stdlog}

	if *s3conf != "" {
		s3b, err := bucket.NewS3BucketFromFile(*s3conf)
		if err != nil {
			log.Fatalf("bucket.NewS3BucketFromFile: %s", err)
		}

		tmp := fmt.Sprintf("kebab_testing_%x", RandomBytes(4))
		TempS3, err = s3b.Descend(tmp)
		if err != nil {
			log.Fatalf("Descend(%q): %s", tmp, err)
		}
		SkipS3 = false
	}
}

func TearDown() {
	if err := os.RemoveAll(TempDir); err != nil {
		log.Printf("os.RemoveAll(%q): %s", TempDir, err)
	}
	if TempS3 != nil {
		if err := TempS3.Destroy(); err != nil {
			log.Printf("tempS3.Destroy(): %s", err)
		}
	}
}

func TempFileBucket(name string) bucket.Bucket {
	dir := filepath.Join(TempDir, name)
	b, err := bucket.NewFileBucket(dir)
	if err != nil {
		panic(err)
	}
	return b
}

func TempS3Bucket(name string) bucket.Bucket {
	b, err := TempS3.Descend(name)
	if err != nil {
		panic(err)
	}
	return b
}

func Upgrade(b bucket.Bucket) bucket.Bucket {
	key := secretkey.New()
	return bucket.NewRecoverableBucket(
		bucket.NewEncryptedBucket(b, key),
		PromptLog,
	)
}

func RandomBytes(n int) []byte {
	x := make([]byte, n)
	if _, err := rand.Read(x); err != nil {
		panic(err)
	}
	return x
}

func HashDir(dir string) []byte {
	h := sha256.New()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			h.Write(data)
		}
		return HashFileInfo(h, info)
	})
	if err != nil {
		log.Fatalf("filepath.Walk failed: %s", err)
	}
	return h.Sum(nil)
}

func HashFileInfo(h hash.Hash, info os.FileInfo) error {
	if _, err := io.WriteString(h, info.Name()); err != nil {
		return err
	}
	if err := binary.Write(h, binary.BigEndian, info.Size()); err != nil {
		return err
	}
	if err := binary.Write(h, binary.BigEndian, info.ModTime().Unix()); err != nil {
		return err
	}
	return nil
}
