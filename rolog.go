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
	// ArchiveFileFormat is the format for old files being rotated out.
	ArchiveFileFormat = "%s-2006-01-02-150405.log"
	// CurrentFilename is the name of the file currently being written.
	CurrentFilename = "%s.log"
)

// Rolog is an io.WriteCloser that writes logs to a single master file and
// periodically pauses to rename the current file for archival, creating a new
// file to continue writing.
type Rolog struct {
	// f is the current file being written
	f *os.File
	// mu is used to synchronize the rotation process and prevent logging during
	// the rename/create window
	mu sync.Mutex
	// interval is how often we should rotate the logs
	interval time.Duration
	// path is the full path to the current file
	path string
	// done is used to signal that our Rolog should stop its main run loop
	done chan int
	// err is used to store any error during rotate. Not currently used.
	err chan error
	// name is the base name of the log file
	name string
}

// Write satisfies io.Writer. It syncs on every write to prevent the visible log
// from being stale while we wait for a flush to disk.
func (r *Rolog) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer func() {
		r.f.Sync()
		r.mu.Unlock()
	}()

	return fmt.Fprintf(r.f, string(p))
}

// Rotate pauses logging switch from the current file to a new one. It moves the
// current file to an archive file by renaming it according to the template and
// creates a new file handle to continue logging.
func (r *Rolog) Rotate() error {
	var (
		err     error
		newPath = filepath.Join(filepath.Dir(r.path), r.fname())
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

// fname returns the canonical name for an archive file.
func (r *Rolog) fname() string {
	return fmt.Sprintf(time.Now().Format(ArchiveFileFormat), r.name)
}

// Close satisfies io.Closer. It performs a final sync prior to closing the
// current file, then signals our run loop to quit.
func (r *Rolog) Close() error {
	r.mu.Lock()
	defer func() {
		r.mu.Unlock()
		r.done <- 1
	}()

	r.f.Sync()
	return r.f.Close()
}

// New creates a Rolog instance which writes files into the given directory. It
// uses the provided name as a base for naming the log files, and rotates them
// on the schedule provided as interval. Note that we automatically set the
// output of log to the new Rolog.
//
// The returned Rolog is not already running, and its Run method must be invoked
// manually.
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

// StartNew calls New, but also starts the Rolog automatically.
func StartNew(dir, name string, interval time.Duration) (*Rolog, error) {
	r, err := New(dir, name, interval)
	if err != nil {
		return nil, errors.Wrap(err, "could not start log rotator")
	}

	r.Run()

	return r, nil
}

// Run starts the Rolog loop in a separate goroutine.
func (r *Rolog) Run() {
	go r.run()
}

// run simply waits for the provided interval and rotates the logs when it is
// reached.
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
