//go:build windows

package windows

import (
	"os"
	"syscall"
	"unsafe"
)

// TryInstanceLock locks the first byte until the file handle is closed. A busy
// lock is reported separately from errors so callers never infer ownership
// from a failed operation. Windows releases it if the process terminates.
func TryInstanceLock(file *os.File) (bool, error) {
	const lockfileFailImmediately = 1
	const lockfileExclusiveLock = 2
	var overlapped syscall.Overlapped
	proc := kernel32.NewProc("LockFileEx")
	ok, _, err := proc.Call(file.Fd(), lockfileFailImmediately|lockfileExclusiveLock,
		0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if ok != 0 {
		return true, nil
	}
	if err == syscall.Errno(33) { // ERROR_LOCK_VIOLATION means another process owns it.
		return false, nil
	}
	return false, err
}
