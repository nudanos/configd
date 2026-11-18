// Package startuplock coordinates writers of the startup configuration.
package startuplock

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

const Directory = "/run/nudanos-startup"
const Timeout = 10 * time.Second

// Acquire returns the open locked file. Close it to release the lock; never unlink it.
func Acquire(dir string, timeout time.Duration) (*os.File, error) {
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		return nil, fmt.Errorf("create startup lock directory: %w", err)
	}
	dfd, err := syscall.Open(dir, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open startup lock directory: %w", err)
	}
	defer syscall.Close(dfd)
	var st syscall.Stat_t
	if err = syscall.Fstat(dfd, &st); err != nil {
		return nil, err
	}
	if st.Uid != uint32(os.Geteuid()) || st.Mode&07777 != 0700 {
		return nil, fmt.Errorf("unsafe startup lock directory metadata")
	}
	fd, err := syscall.Openat(dfd, "startup.lock", syscall.O_RDWR|syscall.O_CREAT|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, fmt.Errorf("open startup lock: %w", err)
	}
	f := os.NewFile(uintptr(fd), "startup.lock")
	if err = syscall.Fstat(fd, &st); err != nil {
		f.Close()
		return nil, err
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFREG || st.Uid != uint32(os.Geteuid()) || st.Mode&07777 != 0600 || st.Nlink != 1 {
		f.Close()
		return nil, fmt.Errorf("unsafe startup lock metadata")
	}
	deadline := time.Now().Add(timeout)
	for {
		err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN && err != syscall.EINTR {
			f.Close()
			return nil, fmt.Errorf("lock startup: %w", err)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			f.Close()
			return nil, fmt.Errorf("timed out waiting for startup lock")
		}
		if remaining > 50*time.Millisecond {
			remaining = 50 * time.Millisecond
		}
		time.Sleep(remaining)
	}
}
