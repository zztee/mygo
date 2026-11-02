//go:build !js && !darwin && !linux && !windows

package mygo

import (
	"errors"
	"os"
)

func tryCheckedInstanceLock(file *os.File) (bool, error) {
	return false, errors.New("strict single-instance ownership is unsupported on this platform")
}
