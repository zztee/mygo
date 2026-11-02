package platform

// SpellingRange is a misspelling in UTF-16 code units, as native text APIs
// count them. The public API translates these into UTF-8 byte offsets.
type SpellingRange struct{ Location, Length int }

// SpellCheckingBackend lets a test backend supply a spelling session. Native
// creation uses a separate function, so even method metadata stays out of
// applications that never opt into the feature.
type SpellCheckingBackend interface {
	NewSpellChecker() (SpellChecker, error)
}

// SpellChecker owns one document's ignored words. Methods run on the main
// thread. Check submits background work and invokes done exactly once from
// any thread; it must not wait for checking to finish. Close is idempotent
// and permits in-flight native callbacks to finish without using freed data.
type SpellChecker interface {
	Check(text, language string, done func([]SpellingRange, error))
	Suggestions(word, language string) ([]string, error)
	Learn(word string) error
	Unlearn(word string) error
	HasLearned(word string) (bool, error)
	Ignore(word string) error
	Close()
}
