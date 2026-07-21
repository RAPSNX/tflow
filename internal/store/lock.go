package store

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/rapsnx/tflow/internal/diag"
)

// unlockFlock releases the advisory lock's flock(2) hold. It is a variable
// so tests can force a release failure without relying on OS-specific
// flock-failure conditions.
var unlockFlock = func(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

// AcquireAppStateLock acquires the advisory lock shared by state mutations.
// The caller must invoke the returned function to release it.
func AcquireAppStateLock(statePath string) (func() error, error) {
	lockPath := statePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o600); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			diag.Warnf("close app-state lock file %q after chmod failure: %v", lockPath, closeErr)
		}
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			diag.Warnf("close app-state lock file %q after flock failure: %v", lockPath, closeErr)
		}
		return nil, err
	}
	return func() error {
		unlockErr := unlockFlock(file)
		closeErr := file.Close()
		if unlockErr != nil {
			return unlockErr
		}
		return closeErr
	}, nil
}
