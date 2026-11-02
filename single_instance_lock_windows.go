package mygo

import (
	"os"

	winapi "github.com/egoist/mygo/internal/windows"
)

func tryCheckedInstanceLock(file *os.File) (bool, error) {
	return winapi.TryInstanceLock(file)
}
