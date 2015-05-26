package bucket_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/davidlazar/kebab/bucket"
	"github.com/davidlazar/kebab/internal/testutil"
)

func TestMain(m *testing.M) {
	testutil.Setup()
	r := m.Run()
	testutil.TearDown()
	os.Exit(r)
}

func TestFileBucket(t *testing.T) {
	b := &TestBucket{
		bucket: testutil.TempFileBucket("FileBucket"),
		t:      t,
	}
	b.allTests()
}

func TestUpgradedFileBucket(t *testing.T) {
	b := &TestBucket{
		bucket: testutil.Upgrade(testutil.TempFileBucket("UpgradedFileBucket")),
		t:      t,
	}
	b.allTests()
}

func TestS3Bucket(t *testing.T) {
	if testutil.SkipS3 {
		t.SkipNow()
	}
	b := &TestBucket{
		bucket: testutil.TempS3Bucket("S3Bucket"),
		t:      t,
	}
	b.allTests()
}

func TestUpgradedS3Bucket(t *testing.T) {
	if testutil.SkipS3 {
		t.SkipNow()
	}
	b := &TestBucket{
		bucket: testutil.Upgrade(testutil.TempS3Bucket("UpgradedS3Bucket")),
		t:      t,
	}
	b.allTests()
}

type TestBucket struct {
	bucket bucket.Bucket
	t      *testing.T
}

// expects the bucket to be empty
func (b *TestBucket) basicTests() {
	b.CheckList(nil, nil)

	b.CheckGetNonexistent("nonexistent")

	data := make([]byte, 256)
	rand.Read(data)

	key0 := "foo"
	b.CheckPut(key0, data)
	b.CheckGet(key0, data)
	b.CheckList([]string{key0}, nil)

	// Overwrite key
	data2 := make([]byte, 512)
	rand.Read(data2)
	b.CheckPut(key0, data2)
	b.CheckGet(key0, data2)
	b.CheckList([]string{key0}, nil)

	key1 := "世界 "
	b.CheckPut(key1, data)
	b.CheckGet(key1, data)
	b.CheckList([]string{key0, key1}, nil)

	// test nil data
	key2 := "nothing"
	b.CheckPut(key2, nil)
	b.CheckGet(key2, nil)
	b.CheckList([]string{key0, key2, key1}, nil)

	b.CheckDestroy()
	b.CheckList(nil, nil)
	b.CheckDestroy()
	b.CheckList(nil, nil)
	b.CheckGetNonexistent(key0)
	b.CheckGetNonexistent(key1)
}

func (b *TestBucket) allTests() {
	b.basicTests()

	pkey0 := "parent-foo"
	b.CheckPut(pkey0, []byte("hello world "))
	b.CheckGet(pkey0, []byte("hello world "))

	data := make([]byte, 256)
	rand.Read(data)

	childName := fmt.Sprintf("TestBucket-%x", data[0:4])
	child := b.CheckDescend(childName)
	child.basicTests()

	ckey0 := "child-bar"
	child.CheckPut(ckey0, data)
	child.CheckGet(ckey0, data)
	child.CheckList([]string{ckey0}, nil)

	b.CheckList([]string{pkey0}, []string{childName})

	// Get child key via parent bucket
	pckey0 := filepath.Join(childName, ckey0) // TODO S3Bucket will break on Windows
	b.CheckGet(pckey0, data)

	// Put child key via parent bucket
	ckey1 := "child-qux"
	pckey1 := filepath.Join(childName, ckey1)
	b.CheckPut(pckey1, data)
	child.CheckGet(ckey1, data)

	child.CheckList([]string{ckey0, ckey1}, nil)

	b.CheckList([]string{pkey0}, []string{childName})
	child.CheckDestroy()
	b.CheckList([]string{pkey0}, nil)
	child.CheckList(nil, nil)

	b.CheckDestroy()
	b.CheckList(nil, nil)
}

func (b *TestBucket) CheckPut(key string, data []byte) {
	if err := b.bucket.Put(key, data); err != nil {
		b.t.Fatalf("Put(%q) failed: %s", key, err)
	}
}

func (b *TestBucket) CheckGet(key string, expected []byte) {
	actual, err := b.bucket.Get(key)
	if err != nil {
		b.t.Fatalf("Get(%q) failed: %s", key, err)
	}
	if !bytes.Equal(actual, expected) {
		b.t.Fatalf("Get(%q):\nexpected: %v\nactually: %v", key, expected, actual)
	}
}

func (b *TestBucket) CheckGetNonexistent(key string) {
	if _, err := b.bucket.Get(key); !bucket.IsNotExist(err) {
		b.t.Fatalf("Get(%q): expected nonexistent key error, got %q", key, err)
	}
}

func (b *TestBucket) CheckList(expectedKeys []string, expectedChildren []string) {
	keys, children, err := b.bucket.List()
	if err != nil {
		b.t.Fatalf("List failed: %s", err)
	}
	if !reflect.DeepEqual(keys, expectedKeys) || !reflect.DeepEqual(children, expectedChildren) {
		b.t.Fatalf(
			"List:\nexpected: (%v, %v)\nactually: (%v, %v)",
			expectedKeys, expectedChildren, keys, children,
		)
	}
}

func (b *TestBucket) CheckDescend(child string) *TestBucket {
	bb, err := b.bucket.Descend(child)
	if err != nil {
		b.t.Fatalf("Descend(%q) failed: %s", child, err)
	}
	return &TestBucket{
		bucket: bb,
		t:      b.t,
	}
}

func (b *TestBucket) CheckDestroy() {
	if err := b.bucket.Destroy(); err != nil {
		b.t.Fatalf("Destroy() failed: %s", err)
	}
}
