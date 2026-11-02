package mygo

import (
	"context"
	"errors"
	"sync"
	"unicode/utf8"

	"github.com/egoist/mygo/internal/platform"
)

// ErrSpellCheckerClosed means the document's spelling session has closed.
var ErrSpellCheckerClosed = errors.New("mygo: spell checker closed")

// SpellingRange identifies a misspelled word by UTF-8 byte offsets in the
// exact string passed to Check. End is exclusive. Apps should discard stale
// results when that string's document revision changes.
type SpellingRange struct{ Start, End int }

// SpellChecker is an opt-in, document-scoped system spelling session. It is
// safe from any goroutine. macOS uses NSSpellChecker; other platforms return
// errors.ErrUnsupported from NewSpellChecker. No dictionary is downloaded.
// Learning changes the user's system dictionary; Ignore lasts for this
// session only. Close the session when its document closes.
type SpellChecker struct {
	// Native operations and session state belong to the main thread.
	native  platform.SpellChecker
	closed  bool
	pending map[*spellingRequest]struct{}
	offQuit func()
}

// linkedSpellCheckers is registered only by the opt-in constructor. The
// accepted quit phase invokes it before the native event loop stops.
var linkedSpellCheckers func()
var spellCheckers map[*SpellChecker]struct{}

type spellingRequest struct{ cancel func() }

// NewSpellChecker creates a session for one document. Like other native
// APIs it needs App.Run; on the main thread before Run it panics.
func NewSpellChecker() (*SpellChecker, error) {
	needsApp("NewSpellChecker")
	var checker *SpellChecker
	err := errLoopStopped
	onMain(func() {
		if App.quitting {
			return
		}
		var native platform.SpellChecker
		if factory, ok := backend().(platform.SpellCheckingBackend); ok {
			native, err = factory.NewSpellChecker()
		} else {
			native, err = newNativeSpellChecker()
		}
		if err == nil {
			checker = &SpellChecker{native: native, pending: make(map[*spellingRequest]struct{})}
			if spellCheckers == nil {
				spellCheckers = make(map[*SpellChecker]struct{})
			}
			spellCheckers[checker] = struct{}{}
			linkedSpellCheckers = func() {
				for session := range spellCheckers {
					session.Close()
				}
			}
			checker.offQuit = App.OnQuit(checker.Close)
		}
	})
	return checker, err
}

// Check submits background system checking. language is a native language
// identifier (for example en_US), or empty for the system spelling language.
// It returns without results on cancellation or Close; native work may still
// finish later. On the main thread it pumps events while waiting. Apps should
// call from a worker and pass only visible/changed paragraphs, not an entire
// large document. Markdown/code filtering belongs to the app.
func (s *SpellChecker) Check(ctx context.Context, text, language string) ([]SpellingRange, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !utf8.ValidString(text) {
		return nil, errors.New("mygo: spelling text is not valid UTF-8")
	}
	type result struct {
		ranges []SpellingRange
		err    error
	}
	ch := make(chan result, 1)
	var once sync.Once
	finish := func(r result) { once.Do(func() { deliver(ch, r) }) }
	stop := context.AfterFunc(ctx, func() { finish(result{err: ctx.Err()}) })
	defer stop()
	request := &spellingRequest{cancel: func() { finish(result{err: ErrSpellCheckerClosed}) }}
	submit := func() {
		if s.closed {
			request.cancel()
			return
		}
		if err := ctx.Err(); err != nil {
			finish(result{err: err})
			return
		}
		s.pending[request] = struct{}{}
		s.native.Check(text, language, func(ranges []platform.SpellingRange, err error) {
			if err != nil {
				finish(result{err: err})
				return
			}
			converted, err := spellingByteRanges(text, ranges)
			finish(result{ranges: converted, err: err})
		})
	}
	// Neither submission nor cleanup makes a worker wait for the UI: quit
	// listeners may cancel and join that worker while they own the thread.
	if isMainThread() {
		submit()
	} else if !postMain(submit) {
		finish(result{err: errLoopStopped})
	}
	r := await(ch)
	postMain(func() { delete(s.pending, request) })
	return r.ranges, r.err
}

func spellingByteRanges(text string, ranges []platform.SpellingRange) ([]SpellingRange, error) {
	// -1 marks the middle of a surrogate pair, which cannot delimit UTF-8.
	bytes := make([]int, 1, len(text)+1)
	for i, r := range text {
		if r > 0xffff {
			bytes = append(bytes, -1)
		}
		bytes = append(bytes, i+utf8.RuneLen(r))
	}
	out := make([]SpellingRange, 0, len(ranges))
	for _, r := range ranges {
		if r.Location < 0 || r.Length <= 0 || r.Location >= len(bytes) || r.Length > len(bytes)-1-r.Location || bytes[r.Location] < 0 || bytes[r.Location+r.Length] < 0 {
			return nil, errors.New("mygo: invalid native spelling range")
		}
		out = append(out, SpellingRange{bytes[r.Location], bytes[r.Location+r.Length]})
	}
	return out, nil
}

func (s *SpellChecker) operation(fn func() error) error {
	err := errLoopStopped
	onMain(func() {
		if s.closed {
			err = ErrSpellCheckerClosed
			return
		}
		err = fn()
	})
	return err
}

// Suggestions returns system replacements for a single word. This explicit
// menu operation is synchronous; it never checks the rest of the document.
func (s *SpellChecker) Suggestions(word, language string) (words []string, err error) {
	err = s.operation(func() error { var e error; words, e = s.native.Suggestions(word, language); return e })
	return
}

// Learn adds a word to the user's persistent system dictionary.
func (s *SpellChecker) Learn(word string) error {
	return s.operation(func() error { return s.native.Learn(word) })
}

// Unlearn removes a word previously learned by the system checker.
func (s *SpellChecker) Unlearn(word string) error {
	return s.operation(func() error { return s.native.Unlearn(word) })
}

// HasLearned reports whether the word is in the learned system dictionary.
func (s *SpellChecker) HasLearned(word string) (learned bool, err error) {
	err = s.operation(func() error { var e error; learned, e = s.native.HasLearned(word); return e })
	return
}

// Ignore suppresses a word for this document session only.
func (s *SpellChecker) Ignore(word string) error {
	return s.operation(func() error { return s.native.Ignore(word) })
}

// Close releases the session and unblocks pending checks. It is idempotent.
func (s *SpellChecker) Close() {
	onMain(func() {
		if s.closed {
			return
		}
		s.closed = true
		if s.offQuit != nil {
			s.offQuit()
			s.offQuit = nil
		}
		for request := range s.pending {
			request.cancel()
		}
		clear(s.pending)
		delete(spellCheckers, s)
		if len(spellCheckers) == 0 {
			linkedSpellCheckers = nil
		}
		s.native.Close()
	})
}
