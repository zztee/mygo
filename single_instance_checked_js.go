package mygo

import "errors"

// RequestSingleInstanceLockChecked cannot provide process ownership in a browser.
func (a *Application) RequestSingleInstanceLockChecked(identity string) (bool, error) {
	return false, errors.New("mygo: strict single-instance ownership is unsupported in browsers")
}
