package kebab

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/davidlazar/kebab/bucket"
)

type Bucket bucket.Bucket

type TarError struct {
	Err    error
	Stderr string
}

func (e *TarError) Error() string {
	s := fmt.Sprintf("tar error: %s", e.Err.Error())
	if e.Stderr != "" {
		lines := strings.Split(e.Stderr, "\n")
		s += fmt.Sprintf(": %q", lines[0])
	}
	return s
}

func Put(b Bucket, srcPath string, files []string) (int64, error) {
	args := []string{"-c", "-z", "-p"}
	if srcPath != "" {
		args = append(args, "-C", srcPath)
	}
	args = append(args, files...)
	cmd := exec.Command("tar", args...)

	w := NewWriter(b, 64*1024*1024)
	cmd.Stdout = w

	var buf bytes.Buffer
	cmd.Stderr = &buf

	err := cmd.Run()
	if err != nil {
		return w.Size(), &TarError{Err: err, Stderr: string(buf.Bytes())}
	}

	if err := w.Close(); err != nil {
		return w.Size(), fmt.Errorf("Close() failed: %s", err)
	}

	return w.Size(), nil
}

func Get(b Bucket, destPath string) (int64, error) {
	r, err := NewReader(b)
	if err != nil {
		return 0, err
	}

	if err := os.Mkdir(destPath, 0700); err != nil {
		return 0, err
	}

	cmd := exec.Command("tar", "-x", "-z", "-f", "-", "-C", destPath)
	cmd.Stdin = r

	out, err := cmd.CombinedOutput()
	if err != nil {
		return r.Size(), &TarError{Err: err, Stderr: string(out)}
	}

	return r.Size(), nil
}
