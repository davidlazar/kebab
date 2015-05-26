package main

import (
	"fmt"
	golog "log"
	"os"
	"sync"
	"time"

	"github.com/davidlazar/go-crypto/secretkey"
	"github.com/davidlazar/kebab"
	"github.com/davidlazar/kebab/bucket"
)

const version = "0.1"
const shortUsage = "usage: %s <flags>\n"
const longUsage = `
List bucket contents:

	-bucket <bucket> -key <file>

Get/Put files:

	-bucket <bucket> -key <file> <commands>

	where <commands> is at least one of:
	
	-get <id>
	-put <id> <file>...
	-putfrom <id> <dir> <file>...

	The -putfrom command puts files relative to <dir> by invoking
	the tar command with the flag -C <dir>

	Multiple commands are executed concurrently.

Delete backups:

	-bucket <bucket> -key -delete <id>...

Generate key file or update passphrase:

	-keygen -key <file>

Print help or version:

	-help | -version

Buckets: <bucket> is a directory or a JSON file describing an S3 bucket.
`

var log *golog.Logger
var plog *bucket.PromptLogger

func init() {
	log = golog.New(os.Stderr, "\nkebab: ", 0)
	plog = &bucket.PromptLogger{Logger: log}
}

type cmdKind int

const (
	cmdPut cmdKind = iota
	cmdPutFrom
	cmdGet
)

type Command struct {
	kind cmdKind
	args []string
}

func (c *Command) Run(b bucket.Bucket) (int64, error) {
	childName := c.args[0]
	child, err := b.Descend(childName)
	if err != nil {
		return 0, fmt.Errorf("error descending into bucket %q: %s", childName, err)
	}

	switch c.kind {
	case cmdPut:
		return kebab.Put(child, "", c.args[1:])
	case cmdPutFrom:
		return kebab.Put(child, c.args[1], c.args[2:])
	case cmdGet:
		return kebab.Get(child, childName)
	default:
		return 0, fmt.Errorf("unexpected command type: %d", c.kind)
	}
}

func (c *Command) String() string {
	switch c.kind {
	case cmdPut:
		return fmt.Sprintf("-put %s", c.args[0])
	case cmdPutFrom:
		return fmt.Sprintf("-putfrom %s", c.args[0])
	case cmdGet:
		return fmt.Sprintf("-get %s", c.args[0])
	default:
		return "unknown command"
	}
}

func main() {
	c, err := parseArgs(os.Args[1:])
	if err != nil {
		log.Fatalf("usage error: %s", err)
	}

	if c.help {
		fmt.Printf(shortUsage, os.Args[0])
		fmt.Print(longUsage)
		return
	}

	if c.version {
		fmt.Println(version)
		return
	}

	if c.keygen {
		if err = Keygen(c.keyPath); err != nil {
			log.Fatalf("keygen error: %s", err)
		}
		return
	}

	bb, err := openBucket(c.bucketPath)
	if err != nil {
		log.Fatalf("error opening bucket: %s", err)
	}

	key, err := secretkey.ReadFile(c.keyPath)
	if err != nil {
		log.Fatalf("error reading key file: %s", err)
	}

	b := upgradeBucket(bb, key)

	if len(c.deletes) > 0 {
		err := deleteBuckets(b, c.deletes)
		if err != nil {
			log.Fatalf("error deleting backups: %s", err)
		}
		return
	}

	if len(c.commands) == 0 {
		_, children, err := b.List()
		if err != nil {
			log.Fatalf("error listing bucket: %s", err)
		}
		for _, child := range children {
			fmt.Println(child)
		}
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(c.commands))
	for _, cmd := range c.commands {
		go func(cmd Command) {
			defer wg.Done()
			start := time.Now()

			n, err := cmd.Run(b)
			if err != nil {
				plog.Printf("command %q failed: %s", cmd.String(), err.Error())
				return
			}

			elapsed := time.Now().Sub(start)
			rounded := time.Duration(int64(elapsed/time.Millisecond)) * time.Millisecond

			mb := float64(n) / 1e6
			mbs := fmt.Sprintf("%.2f MB/s", mb/elapsed.Seconds())

			plog.Printf("%s success: transferred %.2f MB in %s (%s)", cmd.String(), mb, rounded.String(), mbs)
		}(cmd)
	}
	wg.Wait()
}

type Conf struct {
	help       bool
	version    bool
	keygen     bool
	commands   []Command
	deletes    []string
	bucketPath string
	keyPath    string
}

func parseArgs(args []string) (*Conf, error) {
	conf := &Conf{
		commands: make([]Command, 0, 8),
	}
	for len(args) > 0 {
		s := args[0]
		args = args[1:]
		if len(s) == 0 {
			continue
		}
		if s[0] != '-' || len(s) == 1 {
			return nil, fmt.Errorf("unexpected argument: %s", s)
		}

		var flagArgs []string
		var err error
		switch {
		case s == "-h" || s == "-help" || s == "--help":
			conf.help = true
			return conf, nil
		case s == "-v" || s == "-version":
			conf.version = true
			return conf, nil
		case s == "-keygen":
			conf.keygen = true
		case s == "-key":
			flagArgs, args, err = exactly("-key", 1, args)
			if err != nil {
				return nil, err
			}
			conf.keyPath = flagArgs[0]
		case s == "-bucket":
			flagArgs, args, err = exactly("-bucket", 1, args)
			if err != nil {
				return nil, err
			}
			conf.bucketPath = flagArgs[0]
		case s == "-get":
			flagArgs, args, err = exactly("-get", 1, args)
			if err != nil {
				return nil, err
			}
			conf.commands = append(conf.commands, Command{kind: cmdGet, args: flagArgs})
		case s == "-put":
			flagArgs, args, err = atleast("-put", 2, args)
			if err != nil {
				return nil, err
			}
			conf.commands = append(conf.commands, Command{kind: cmdPut, args: flagArgs})
		case s == "-putfrom":
			flagArgs, args, err = atleast("-putfrom", 3, args)
			if err != nil {
				return nil, err
			}
			conf.commands = append(conf.commands, Command{kind: cmdPutFrom, args: flagArgs})
		case s == "-delete":
			flagArgs, args, err = atleast("-delete", 1, args)
			if err != nil {
				return nil, err
			}
			conf.deletes = append(conf.deletes, flagArgs...)
		default:
			return nil, fmt.Errorf("unrecognized flag: %q", s)
		}
	}
	if conf.keyPath == "" {
		return nil, fmt.Errorf("flag -key required")
	}
	if !conf.keygen && conf.bucketPath == "" {
		return nil, fmt.Errorf("flag -bucket required")
	}
	if len(conf.deletes) > 0 && len(conf.commands) > 0 {
		return nil, fmt.Errorf("can not delete and put/get at the same time")
	}
	return conf, nil
}

func exactly(flag string, count int, args []string) (flagArgs, rest []string, err error) {
	for rest = args; len(rest) > 0 && len(flagArgs) < count; rest = rest[1:] {
		arg := rest[0]
		if len(arg) == 0 {
			continue
		}
		if arg[0] == '-' {
			break
		}
		flagArgs = append(flagArgs, arg)
	}
	if len(flagArgs) < count {
		err = fmt.Errorf("flag %s: expecting %d argument%s", flag, count, plural(count))
	}
	return
}

func atleast(flag string, count int, args []string) (flagArgs, rest []string, err error) {
	for rest = args; len(rest) > 0; rest = rest[1:] {
		arg := rest[0]
		if len(arg) == 0 {
			continue
		}
		if arg[0] == '-' {
			break
		}
		flagArgs = append(flagArgs, arg)
	}
	if len(flagArgs) < count {
		err = fmt.Errorf("flag %s: expecting at least %d argument%s", flag, count, plural(count))
	}
	return
}

func plural(count int) string {
	if count == 1 {
		return ""
	} else {
		return "s"
	}
}
