package bucket

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

func isRecoverable(err error) bool {
	if IsNotExist(err) {
		return false
	}
	if err == ErrAuth {
		return false
	}
	return true
}

type recoverableBucket struct {
	bucket Bucket
	log    *PromptLogger
}

func NewRecoverableBucket(b Bucket, log *PromptLogger) Bucket {
	return &recoverableBucket{
		bucket: b,
		log:    log,
	}
}

func (b *recoverableBucket) Abs(key string) string {
	return b.bucket.Abs(key)
}

func (b *recoverableBucket) Put(key string, data []byte) error {
	for {
		if err := b.bucket.Put(key, data); err == nil {
			return nil
		} else {
			b.log.Printf("Put(%q) failed: %s\n... Retrying in 5 seconds.", key, err)
			time.Sleep(5 * time.Second)
		}

		if err := b.bucket.Put(key, data); err == nil {
			return nil
		} else {
			if !b.log.Retry("Put(%q) failed: %s", key, err) {
				return fmt.Errorf("Put(%q): %s", key, err)
			}
		}
	}
}

// TODO don't try to recover if key is a directory
func (b *recoverableBucket) Get(key string) ([]byte, error) {
	for {
		if data, err := b.bucket.Get(key); err == nil {
			return data, nil
		} else if !isRecoverable(err) {
			return nil, err
		} else {
			b.log.Printf("Get(%q) failed: %s\n... Retrying in 5 seconds.", key, err)
			time.Sleep(5 * time.Second)
		}

		if data, err := b.bucket.Get(key); err == nil {
			return data, nil
		} else {
			if !b.log.Retry("Get(%q) failed: %s", key, err) {
				return nil, fmt.Errorf("Get(%q): %s", key, err)
			}
		}
	}
}

func (b *recoverableBucket) List() (keys, children []string, err error) {
	return b.bucket.List()
}

func (b *recoverableBucket) Descend(child string) (Bucket, error) {
	bb, err := b.bucket.Descend(child)
	if err != nil {
		return nil, err
	}
	return NewRecoverableBucket(bb, b.log), nil
}

// TODO add recovery?
func (b *recoverableBucket) Destroy() error {
	return b.bucket.Destroy()
}

type PromptLogger struct {
	mu     sync.Mutex
	Logger *log.Logger
}

func (l *PromptLogger) output(s string) {
	l.mu.Lock()
	l.Logger.Println(s)
	l.mu.Unlock()
}

func (l *PromptLogger) Printf(format string, v ...interface{}) {
	l.output(fmt.Sprintf(format, v...))
}

func (l *PromptLogger) Retry(format string, v ...interface{}) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Logger.Printf(format, v...)

	fmt.Fprintf(os.Stderr, "--> Retry? [Y/n]: ")
	buf := bufio.NewReader(os.Stdin)
	line, err := buf.ReadString('\n')
	if err != nil || line == "" || line[0] == 'n' || line[0] == 'N' {
		return false
	}
	return true
}
