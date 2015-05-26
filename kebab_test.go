package kebab

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidlazar/kebab/bucket"
	"github.com/davidlazar/kebab/internal/testutil"
)

const Megabyte = 1024 * 1024

var dataDir string

func TestMain(m *testing.M) {
	testutil.Setup()

	dataDir = filepath.Join(testutil.TempDir, "data")
	if err := os.Mkdir(dataDir, 0700); err != nil {
		log.Fatalf("os.Mkdir: %s", err)
	}

	lorem := bytes.Repeat([]byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.\n"), 1*Megabyte)
	writeFile(dataDir, "lorem.txt", lorem)

	for i := 0; i < 5; i++ {
		x := testutil.RandomBytes(i * Megabyte)
		writeFile(dataDir, fmt.Sprintf("%dMB.data", i), x)
	}

	writeFile(dataDir, "80MB.data", testutil.RandomBytes(80*Megabyte))

	r := m.Run()

	testutil.TearDown()

	os.Exit(r)
}

var (
	fsBucket bucket.Bucket
	s3Bucket bucket.Bucket
	fsPutOk  bool
	s3PutOk  bool
)

func TestFSPut(t *testing.T) {
	fsBucket = testutil.Upgrade(testutil.TempFileBucket("FSPutGet"))

	_, err := Put(fsBucket, testutil.TempDir, []string{"data"})
	if err != nil {
		t.Fatalf("Put failed: %s", err)
	}
	fsPutOk = true
}

func TestFSGet(t *testing.T) {
	if !fsPutOk {
		t.Skipf("did not run TestFSPut")
	}

	destDir := filepath.Join(testutil.TempDir, "dest")
	if _, err := Get(fsBucket, destDir); err != nil {
		t.Fatalf("Get failed: %s", err)
	}

	h1 := testutil.HashDir(dataDir)
	h2 := testutil.HashDir(filepath.Join(destDir, "data"))

	if !bytes.Equal(h1, h2) {
		t.Fatalf("directories differ: %q, %q", dataDir, destDir)
	}

	if err := os.RemoveAll(destDir); err != nil {
		t.Fatalf("os.RemoveAll: %s", err)
	}
}

func TestS3Put(t *testing.T) {
	if testutil.SkipS3 {
		t.SkipNow()
	}

	s3Bucket = testutil.Upgrade(testutil.TempS3Bucket("S3PutGet"))

	_, err := Put(s3Bucket, testutil.TempDir, []string{"data"})
	if err != nil {
		t.Fatalf("Put failed: %s", err)
	}
	s3PutOk = true
}

func TestS3Get(t *testing.T) {
	if testutil.SkipS3 {
		t.SkipNow()
	}
	if !s3PutOk {
		t.Skipf("did not run TestS3Put")
	}

	destDir := filepath.Join(testutil.TempDir, "dest-s3")
	if _, err := Get(s3Bucket, destDir); err != nil {
		t.Fatalf("Get failed: %s", err)
	}

	h1 := testutil.HashDir(dataDir)
	h2 := testutil.HashDir(filepath.Join(destDir, "data"))

	if !bytes.Equal(h1, h2) {
		t.Fatalf("directories differ: %q, %q", dataDir, destDir)
	}

	if err := os.RemoveAll(destDir); err != nil {
		t.Fatalf("os.RemoveAll: %s", err)
	}
}

var (
	benchBucket bucket.Bucket
	benchPutOk  bool
)

func BenchmarkS3Put(b *testing.B) {
	if testutil.SkipS3 {
		b.SkipNow()
	}

	benchBucket = testutil.Upgrade(testutil.TempS3Bucket("BenchmarkPutS3"))

	for i := 0; i < b.N; i++ {
		n, err := Put(benchBucket, testutil.TempDir, []string{"data"})
		if err != nil {
			b.Fatalf("Put failed: %s", err)
		}
		b.SetBytes(n)
	}
	benchPutOk = true
}

func BenchmarkS3Get(b *testing.B) {
	if testutil.SkipS3 {
		b.SkipNow()
	}
	if !benchPutOk {
		b.Skipf("did not run BenchmarkS3Put")
	}

	destDir := filepath.Join(testutil.TempDir, "dest-s3-benchmark")

	for i := 0; i < b.N; i++ {
		n, err := Get(benchBucket, destDir)
		if err != nil {
			b.Fatalf("Get failed: %s", err)
		}
		b.SetBytes(n)
	}
}

func writeFile(top string, name string, data []byte) {
	path := filepath.Join(top, name)
	if err := ioutil.WriteFile(path, data, 0600); err != nil {
		log.Fatalf("ioutil.WriteFile: %s", err)
	}
}
