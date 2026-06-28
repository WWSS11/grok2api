//go:build windows

package main

import (
	"os"
	"sync"
)

var (
	schedulerLockMu sync.Mutex
	schedulerLockFd *os.File
)

// tryAcquireSchedulerLock on Windows always returns true (no flock available).
func tryAcquireSchedulerLock(lockPath string) bool {
	return true
}

// releaseSchedulerLock on Windows is a no-op.
func releaseSchedulerLock() {
	schedulerLockMu.Lock()
	defer schedulerLockMu.Unlock()
	schedulerLockFd = nil
}
