package startuplock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockContentionAndRelease(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "lockdir")
	first, err := Acquire(dir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := Acquire(dir, 120*time.Millisecond); err == nil {
		t.Fatal("contended lock unexpectedly acquired")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := Acquire(dir, time.Second)
	if err != nil {
		t.Fatalf("released lock unavailable: %v", err)
	}
	next.Close()
}

func TestRejectUnsafeDirectoryAndFile(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(string) error
	}{
		{"directory symlink", func(d string) error {
			target := d + "-target"
			if err := os.Mkdir(target, 0700); err != nil {
				return err
			}
			return os.Symlink(target, d)
		}},
		{"directory permissions", func(d string) error { return os.Mkdir(d, 0755) }},
		{"file symlink", func(d string) error {
			if err := os.Mkdir(d, 0700); err != nil {
				return err
			}
			return os.Symlink("/dev/null", filepath.Join(d, "startup.lock"))
		}},
		{"file permissions", func(d string) error {
			if err := os.Mkdir(d, 0700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(d, "startup.lock"), nil, 0644)
		}},
		{"file hardlink", func(d string) error {
			if err := os.Mkdir(d, 0700); err != nil {
				return err
			}
			p := filepath.Join(d, "startup.lock")
			if err := os.WriteFile(p, nil, 0600); err != nil {
				return err
			}
			return os.Link(p, filepath.Join(d, "other"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := filepath.Join(t.TempDir(), "lockdir")
			if err := tc.prepare(d); err != nil {
				t.Fatal(err)
			}
			f, err := Acquire(d, 100*time.Millisecond)
			if err == nil {
				f.Close()
				t.Fatal("unsafe lock accepted")
			}
		})
	}
}
