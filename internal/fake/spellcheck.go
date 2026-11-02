package fake

import "github.com/egoist/mygo/internal/platform"

func (b *Backend) NewSpellChecker() (platform.SpellChecker, error) {
	if b.SpellCheckerFactory != nil {
		return b.SpellCheckerFactory()
	}
	return nil, platform.ErrUnsupported
}
