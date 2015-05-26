package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/davidlazar/go-crypto/secretkey"
	"github.com/davidlazar/kebab/bucket"
)

func openBucket(bucketPath string) (bucket.Bucket, error) {
	file, err := os.Open(bucketPath)
	if err != nil {
		return nil, err
	}
	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return bucket.NewFileBucket(bucketPath)
	} else {
		b, err := bucket.NewS3BucketFromFile(bucketPath)
		if err != nil {
			return nil, fmt.Errorf("bucket.NewS3BucketFromFile: %s", err)
		}
		return b, nil
	}
}

func upgradeBucket(b bucket.Bucket, key *secretkey.Key) bucket.Bucket {
	return bucket.NewRecoverableBucket(bucket.NewEncryptedBucket(b, key), plog)
}

func deleteBuckets(root bucket.Bucket, names []string) error {
	_, children, err := root.List()
	if err != nil {
		return fmt.Errorf("List: %s", err)
	}
	childMap := make(map[string]bool)
	for _, child := range children {
		childMap[child] = true
	}

	for _, name := range names {
		if _, ok := childMap[name]; !ok {
			fmt.Printf("\n%q not found. Skipping.\n", name)
			continue
		}

		child, err := root.Descend(name)
		if err != nil {
			return fmt.Errorf("Descend(%q): %s", name, err)
		}

		keys, children, err := child.List()
		if err != nil {
			return fmt.Errorf("List: %s", err)
		}

		fmt.Printf("\nGoing to delete %q:\n", name)
		for _, s := range summary(keys) {
			fmt.Println("  ", s)
		}
		for _, s := range summary(children) {
			fmt.Println("  ", s)
		}
		fmt.Printf("Continue? [y/N]: ")

		buf := bufio.NewReader(os.Stdin)
		line, err := buf.ReadString('\n')
		if err != nil {
			return fmt.Errorf("ReadString: %s", err)
		}
		if line == "" || (line[0] != 'y' && line[0] != 'Y') {
			fmt.Println("Delete cancelled!")
			return nil
		}

		err = child.Destroy()
		if err != nil {
			return fmt.Errorf("Destroy: %s", err)
		}

		fmt.Printf("Deleted %q\n", name)
	}
	return nil
}

func summary(xs []string) []string {
	if len(xs) <= 6 {
		return xs
	}
	r := xs[0:3]
	r = append(r, "...")
	r = append(r, xs[len(xs)-3:]...)
	return r
}
