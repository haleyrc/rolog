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
	ArchiveFileFormat = "invsvc-2006-01-02-150405.log"
	CurrentFilename   = "invsvc.log"
)

type Rolog struct {
	f        *os.File
	mu       sync.Mutex
	interval time.Duration
	path     string
	done     chan int
	err      chan error
}

func (r *Rolog) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return fmt.Fprintf(r.f, string(p))
}

func (r *Rolog) Rotate() error {
	var (
		err     error
		newPath = filepath.Join(filepath.Dir(r.path), fname())
	)

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

func fname() string {
	return time.Now().Format(ArchiveFileFormat)
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

func New(dir string, interval time.Duration) (*Rolog, error) {
	var (
		file = filepath.Join(dir, CurrentFilename)
		r    = &Rolog{}
		err  error
	)

	if _, err = os.Stat(file); err == nil {
		if err = os.Rename(file, fname()); err != nil {
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

func StartNew(dir string, interval time.Duration) (*Rolog, error) {
	r, err := New(dir, interval)
	if err != nil {
		return nil, errors.Wrap(err, "could not start log rotator")
	}

	r.Run()

	return r, nil
}

func (r *Rolog) Run() {
	go func() {
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
	}()
}
