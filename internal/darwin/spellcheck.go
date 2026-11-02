//go:build darwin

package darwin

import (
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/internal/platform"
)

// Typed messages are registered only when an app opts into spelling. NSRange
// occupies two integer registers on both supported Darwin architectures.
var spellingOnce sync.Once
var spellMessages spellingAPI

func spellingMessages() *spellingAPI {
	spellingOnce.Do(func() {
		purego.RegisterFunc(&spellMessages.request, msgSendAddr)
		purego.RegisterFunc(&spellMessages.guesses, msgSendAddr)
		purego.RegisterFunc(&spellMessages.rangeOf, msgSendAddr)
	})
	return &spellMessages
}

type spellingAPI struct {
	request func(id, objc.SEL, id, nsRange, uint64, id, int, objc.Block) int
	guesses func(id, objc.SEL, nsRange, id, id, int) id
	rangeOf func(id, objc.SEL) nsRange
}

type spellChecker struct {
	checker id
	tag     int
	closed  bool
}

// NewSpellChecker creates an opt-in document session on the main thread.
func NewSpellChecker() (platform.SpellChecker, error) {
	var s *spellChecker
	withPool(func() {
		checker := send(class("NSSpellChecker"), "sharedSpellChecker")
		if checker != 0 {
			s = &spellChecker{checker: checker, tag: sendInt(class("NSSpellChecker"), "uniqueSpellDocumentTag")}
		}
	})
	if s == nil {
		return nil, platform.ErrUnsupported
	}
	return s, nil
}

func (s *spellChecker) Check(text, language string, done func([]platform.SpellingRange, error)) {
	api := spellingMessages()
	withPool(func() {
		// purego caches the native callback by signature; each block owns only
		// this request's Go closure and is released by Cocoa after completion.
		block := newBlock(func(_ objc.Block, _ int, results id, _ id, _ int) {
			var ranges []platform.SpellingRange
			withPool(func() {
				for _, result := range arrayItems(results) {
					if send(result, "resultType") != 1<<1 {
						continue
					}
					r := api.rangeOf(result, sel("range"))
					ranges = append(ranges, platform.SpellingRange{Location: int(r.Location), Length: int(r.Length)})
				}
			})
			done(ranges, nil)
		})
		defer block.Release()
		options := spellingOptions(language)
		api.request(s.checker, sel("requestCheckingOfString:range:types:options:inSpellDocumentWithTag:completionHandler:"), nsString(text), nsRange{Location: 0, Length: uint(utf16Len(text))}, 1<<1, options, s.tag, block)
	})
}

func (s *spellChecker) Suggestions(word, language string) (words []string, err error) {
	withPool(func() {
		var lang id
		if language != "" {
			lang = nsString(language)
		}
		array := spellingMessages().guesses(s.checker, sel("guessesForWordRange:inString:language:inSpellDocumentWithTag:"), nsRange{Location: 0, Length: uint(utf16Len(word))}, nsString(word), lang, s.tag)
		for _, value := range arrayItems(array) {
			words = append(words, goString(value))
		}
	})
	return words, nil
}

func (s *spellChecker) Learn(word string) error {
	withPool(func() { send(s.checker, "learnWord:", uintptr(nsString(word))) })
	return nil
}

func (s *spellChecker) Unlearn(word string) error {
	withPool(func() { send(s.checker, "unlearnWord:", uintptr(nsString(word))) })
	return nil
}

func (s *spellChecker) HasLearned(word string) (learned bool, err error) {
	withPool(func() { learned = sendBool(s.checker, "hasLearnedWord:", uintptr(nsString(word))) })
	return learned, nil
}

func (s *spellChecker) Ignore(word string) error {
	withPool(func() { send(s.checker, "ignoreWord:inSpellDocumentWithTag:", uintptr(nsString(word)), uintptr(s.tag)) })
	return nil
}

func (s *spellChecker) Close() {
	if s.closed {
		return
	}
	s.closed = true
	// Closing a document discards its ignored words. In-flight requests
	// own their strings/blocks in Cocoa and their results are discarded by
	// the public session, so cleanup need not wait for the event loop.
	s.closeDocument()
}

func (s *spellChecker) closeDocument() {
	withPool(func() { send(s.checker, "closeSpellDocumentWithTag:", uintptr(s.tag)) })
}

// Explicit languages are supplied as an orthography; AppKit has no language
// option key. Omitting orthography preserves the user's system preference.
func spellingOptions(language string) id {
	if language == "" {
		return 0
	}
	orthography := send(class("NSOrthography"), "defaultOrthographyForLanguage:", uintptr(nsString(strings.ReplaceAll(language, "_", "-"))))
	options := send(class("NSMutableDictionary"), "dictionary")
	send(options, "setObject:forKey:", uintptr(orthography), uintptr(constString(libAppKit, "NSTextCheckingOrthographyKey")))
	return options
}
