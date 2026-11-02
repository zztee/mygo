//go:build darwin

package mygo

import (
	"github.com/egoist/mygo/internal/darwin"
	"github.com/egoist/mygo/internal/platform"
)

func newNativeSpellChecker() (platform.SpellChecker, error) { return darwin.NewSpellChecker() }
