//go:build !windows

package main

import (
	"os"
	"sync"
	"syscall"
)

var (
	schedulerLockMu sync.Mutex
	schedulerLockFd *os.File
)

// tryAcquireSchedulerLock performs a non-blocking flock on lockPath.
func tryAcquireSchedulerLock(lockPath string) bool {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return true
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return false
	}
	schedulerLockMu.Lock()
	schedulerLockFd = f
	schedulerLockMu.Unlock()
	return true
}

// releaseSchedulerLock releases the advisory file lock.
func releaseSchedulerLock() {
	schedulerLockMu.Lock()
	defer schedulerLockMu.Unlock()
	if schedulerLockFd == nil {
		return
	}
	_ = syscall.Flock(int(schedulerLockFd.Fd()), syscall.LOCK_UN)
	_ = schedulerLockFd.Close()
	schedulerLockFd = nil
}
