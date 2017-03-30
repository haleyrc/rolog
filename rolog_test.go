package rolog

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"
)

func TestFnameCreatesNames(t *testing.T) {
	now := time.Now()
	year, month, day := now.Date()
	hour, min, sec := now.Clock()

	want := fmt.Sprintf("invsvc-%d-%0.2d-%0.2d-%0.2d%0.2d%0.2d.log", year, month, day, hour, min, sec)
	fmt.Printf("Looking for: %s\n", want)

	got := fname()

	if want != got {
		t.Errorf("Wanted %s, got %s", want, got)
	}
}

func TestNewCreatesARolog(t *testing.T) {
	var want = struct {
		interval time.Duration
	}{
		5 * time.Second,
	}

	dir, err := ioutil.TempDir(".", "tmp")
	if err != nil {
		t.Errorf("unexpected error: %q", err)
		t.FailNow()
	}

	r := New(dir, want.interval)
	defer func() {
		r.Close()
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("could not cleanup temp files: %q", err)
		}
	}()

	if r.interval != want.interval {
		t.Errorf("Wanted interval %s, got %s", want.interval, r.interval)
	}
}

func TestRotateCreatesArchiveAndOpensNew(t *testing.T) {
	dir, err := ioutil.TempDir(".", "tmp")
	if err != nil {
		t.Errorf("unexpected error: %q", err)
		t.FailNow()
	}

	r := New(dir, 5*time.Second)
	defer func() {
		r.Close()
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("could not cleanup temp files: %q", err)
		}
	}()

	{
		if err := r.Rotate(); err != nil {
			t.Errorf("could not rotate: %q", err)
			t.FailNow()
		}

		fi, err := ioutil.ReadDir(dir)
		if err != nil {
			t.Errorf("unexpected error: %q", err)
			t.FailNow()
		}

		if len(fi) != 2 {
			t.Errorf("Wanted 2 files, got %d", len(fi))
		}
	}

	// Wait here just to make sure we get a new filename
	time.Sleep(1 * time.Second)

	{
		if err := r.Rotate(); err != nil {
			t.Errorf("could not rotate: %q", err)
			t.FailNow()
		}

		fi, err := ioutil.ReadDir(dir)
		if err != nil {
			t.Errorf("unexpected error: %q", err)
			t.FailNow()
		}

		if len(fi) != 3 {
			t.Errorf("Wanted 3 files, got %d", len(fi))
		}
	}
}

func TestRunCreatesFilesOnTime(t *testing.T) {
	dir, err := ioutil.TempDir(".", "tmp")
	if err != nil {
		t.Errorf("unexpected error: %q", err)
		t.FailNow()
	}

	r := New(dir, 5*time.Second)
	defer func() {
		r.Close()
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("could not cleanup temp files: %q", err)
		}
	}()

	r.Run()
	log.Println("first")
	time.Sleep(6 * time.Second)
	log.Println("second")
	time.Sleep(7 * time.Second)
	log.Println("third")
	r.Close()

	fi, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Errorf("unexpected error: %q", err)
		t.FailNow()
	}

	if len(fi) != 3 {
		t.Errorf("Wanted 3 files, got %d", len(fi))
	}
}
