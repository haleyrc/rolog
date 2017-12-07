package rolog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const (
	ArchiveFileFormat = "%s-2006-01-02-150405.log"
	CurrentFilename   = "%s.log"
)

type Rolog struct {
	f        *os.File
	mu       sync.Mutex
	interval time.Duration
	path     string
	done     chan int
	err      chan error
	name     string
}

func (r *Rolog) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer func() {
		r.f.Sync()
		r.mu.Unlock()
	}()

	return fmt.Fprintf(r.f, string(p))
}

func (r *Rolog) Rotate() error {
	var (
		err     error
		newPath = filepath.Join(filepath.Dir(r.path), r.fname())
	)
	fmt.Printf("path=%s\n", r.path)
	fmt.Printf("newpath=%s\n", newPath)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.f.Sync()
	r.f.Close()
	if err = os.Rename(r.path, newPath); err != nil {
		return errors.Wrap(err, "could not archive old log file")
	}

	r.f, err = os.Create(r.path)
	if err != nil {
		return errors.Wrap(err, "could not open new log file")
	}

	return nil
}

func (r *Rolog) fname() string {
	return fmt.Sprintf(time.Now().Format(ArchiveFileFormat), r.name)
}

func (r *Rolog) Close() error {
	r.mu.Lock()
	defer func() {
		r.mu.Unlock()
		r.done <- 1
	}()

	r.f.Sync()
	return r.f.Close()
}

func New(dir, name string, interval time.Duration) (*Rolog, error) {
	var (
		file = filepath.Join(dir, fmt.Sprintf(CurrentFilename, name))
		r    = &Rolog{}
		err  error
	)

	r.name = name

	if _, err = os.Stat(file); err == nil {
		if err = os.Rename(file, filepath.Join(dir, r.fname())); err != nil {
			return nil, errors.Wrap(err, "could not archive existing log")
		}
	}

	r.f, err = os.Create(file)
	if err != nil {
		return nil, errors.Wrap(err, "could not create new log")
	}

	r.path = file
	r.interval = interval
	r.done = make(chan int, 1)
	r.err = make(chan error, 1)

	log.SetOutput(r)

	return r, nil
}

func StartNew(dir, name string, interval time.Duration) (*Rolog, error) {
	r, err := New(dir, name, interval)
	if err != nil {
		return nil, errors.Wrap(err, "could not start log rotator")
	}

	r.Run()

	return r, nil
}

func (r *Rolog) Run() {
	go r.run()
}

func (r *Rolog) run() {
	ticker := time.NewTicker(r.interval)
	for {
		select {
		case <-ticker.C:
			if err := r.Rotate(); err != nil {
				r.err <- err
				r.done <- 1
			}
		case <-r.done:
			return
		default:
		}
	}
}
