//go:build !js

package mygo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"testing"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/wait"
)

type spellTestRequest struct {
	text, language string
	done           func([]platform.SpellingRange, error)
}

type spellTestNative struct {
	requests chan spellTestRequest
	learned  map[string]bool
	ignored  map[string]bool
	calls    []string
	closed   int
	err      error
}

func (n *spellTestNative) Check(text, language string, done func([]platform.SpellingRange, error)) {
	n.requests <- spellTestRequest{text, language, done}
}
func (n *spellTestNative) Suggestions(word, language string) ([]string, error) {
	n.calls = append(n.calls, word+":"+language)
	return []string{"spelling", "selling"}, n.err
}
func (n *spellTestNative) Learn(word string) error              { n.learned[word] = true; return n.err }
func (n *spellTestNative) Unlearn(word string) error            { delete(n.learned, word); return n.err }
func (n *spellTestNative) HasLearned(word string) (bool, error) { return n.learned[word], n.err }
func (n *spellTestNative) Ignore(word string) error             { n.ignored[word] = true; return n.err }
func (n *spellTestNative) Close()                               { n.closed++ }

func newTestSpellChecker(t *testing.T) (*SpellChecker, *spellTestNative) {
	t.Helper()
	n := &spellTestNative{requests: make(chan spellTestRequest, 8), learned: map[string]bool{}, ignored: map[string]bool{}}
	onMain(func() { fb.SpellCheckerFactory = func() (platform.SpellChecker, error) { return n, nil } })
	s, err := NewSpellChecker()
	onMain(func() { fb.SpellCheckerFactory = nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, n
}

func TestSpellCheckerBackgroundUnicode(t *testing.T) {
	s, n := newTestSpellChecker(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ranges, err := s.Check(context.Background(), "中😀 speling fin", "en_US")
		if err != nil || !reflect.DeepEqual(ranges, []SpellingRange{{8, 15}}) {
			t.Errorf("ranges=%v, err=%v", ranges, err)
		}
	}()
	request := <-n.requests
	if request.text != "中😀 speling fin" || request.language != "en_US" {
		t.Fatal(request)
	}
	// A request is in flight, but the main thread can still process work.
	onMain(func() {})
	request.done([]platform.SpellingRange{{Location: 4, Length: 7}}, nil)
	<-done
	// A backend accidentally replying twice cannot block or replace results.
	request.done(nil, errors.New("late reply"))
}

func TestSpellCheckerMainThreadWait(t *testing.T) {
	s, n := newTestSpellChecker(t)
	go func() { request := <-n.requests; request.done(nil, nil) }()
	onMain(func() {
		_, err := s.Check(context.Background(), "hello", "")
		if err != nil {
			t.Error(err)
		}
	})
}

func TestSpellCheckerCancelAndClose(t *testing.T) {
	for _, mode := range []string{"cancel", "close"} {
		t.Run(mode, func(t *testing.T) {
			s, n := newTestSpellChecker(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := s.Check(ctx, "speling", ""); done <- err }()
			request := <-n.requests
			want := context.Canceled
			if mode == "close" {
				s.Close()
				s.Close()
				want = ErrSpellCheckerClosed
			} else {
				cancel()
			}
			if err := <-done; !errors.Is(err, want) {
				t.Fatalf("err=%v, want %v", err, want)
			}
			request.done([]platform.SpellingRange{{Location: 0, Length: 7}}, nil)
			if mode == "close" {
				_, err := s.Check(context.Background(), "again", "")
				if !errors.Is(err, ErrSpellCheckerClosed) {
					t.Fatal(err)
				}
				if err := s.Ignore("word"); !errors.Is(err, ErrSpellCheckerClosed) {
					t.Fatal(err)
				}
				onMain(func() {
					if n.closed != 1 {
						t.Errorf("closed %d times", n.closed)
					}
				})
			} else {
				go func() { request := <-n.requests; request.done(nil, nil) }()
				if _, err := s.Check(context.Background(), "again", ""); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSpellCheckerOperationsAndIsolation(t *testing.T) {
	s, n := newTestSpellChecker(t)
	other, otherNative := newTestSpellChecker(t)
	if err := s.Ignore("speling"); err != nil {
		t.Fatal(err)
	}
	if err := s.Learn("myword"); err != nil {
		t.Fatal(err)
	}
	if learned, err := s.HasLearned("myword"); !learned || err != nil {
		t.Fatalf("learned=%v err=%v", learned, err)
	}
	if err := s.Unlearn("myword"); err != nil {
		t.Fatal(err)
	}
	if learned, err := s.HasLearned("myword"); learned || err != nil {
		t.Fatalf("learned=%v err=%v", learned, err)
	}
	words, err := s.Suggestions("speling", "en_GB")
	if err != nil || !reflect.DeepEqual(words, []string{"spelling", "selling"}) {
		t.Fatalf("words=%v err=%v", words, err)
	}
	onMain(func() {
		if !n.ignored["speling"] || len(otherNative.ignored) != 0 || !reflect.DeepEqual(n.calls, []string{"speling:en_GB"}) {
			t.Error("session state or arguments not isolated")
		}
		n.err = errors.New("native failure")
	})
	if _, err := s.Suggestions("word", ""); err == nil || err.Error() != "native failure" {
		t.Fatal(err)
	}
	other.Close()
	if err := s.Ignore("another"); err == nil {
		t.Fatal("native error not propagated")
	}
}

func TestSpellCheckerUnsupportedAndFailedCreation(t *testing.T) {
	if _, err := NewSpellChecker(); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatal(err)
	}
	want := errors.New("factory failed")
	onMain(func() { fb.SpellCheckerFactory = func() (platform.SpellChecker, error) { return nil, want } })
	defer onMain(func() { fb.SpellCheckerFactory = nil })
	if s, err := NewSpellChecker(); s != nil || !errors.Is(err, want) {
		t.Fatalf("checker=%v err=%v", s, err)
	}
}

func TestSpellCheckerNativeFailureAndCanceledInput(t *testing.T) {
	s, n := newTestSpellChecker(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Check(ctx, "word", ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.Check(context.Background(), "\xff", ""); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	if len(n.requests) != 0 {
		t.Fatal("invalid input reached native checker")
	}
	want := errors.New("checking failed")
	go func() { request := <-n.requests; request.done(nil, want) }()
	if _, err := s.Check(context.Background(), "word", ""); !errors.Is(err, want) {
		t.Fatal(err)
	}
}

func TestSpellCheckerRangeConversion(t *testing.T) {
	text := "中😀é test"
	for _, test := range []struct {
		name    string
		native  []platform.SpellingRange
		want    []SpellingRange
		invalid bool
	}{
		{"empty", nil, []SpellingRange{}, false},
		{"unicode", []platform.SpellingRange{{Location: 0, Length: 4}, {Location: 5, Length: 4}}, []SpellingRange{{0, 9}, {10, 14}}, false},
		{"surrogate start", []platform.SpellingRange{{Location: 2, Length: 1}}, nil, true},
		{"surrogate end", []platform.SpellingRange{{Location: 1, Length: 1}}, nil, true},
		{"past end", []platform.SpellingRange{{Location: 9, Length: 1}}, nil, true},
		{"overflow", []platform.SpellingRange{{Location: 1, Length: int(^uint(0) >> 1)}}, nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := spellingByteRanges(text, test.native)
			if (err != nil) != test.invalid || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got=%v err=%v want=%v", got, err, test.want)
			}
		})
	}
}

func TestSpellCheckerConcurrentRequests(t *testing.T) {
	s, n := newTestSpellChecker(t)
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			if _, err := s.Check(context.Background(), "bad", ""); err != nil {
				t.Error(err)
			}
		})
	}
	requests := make([]spellTestRequest, 6)
	for i := range requests {
		requests[i] = <-n.requests
	}
	for i := len(requests) - 1; i >= 0; i-- {
		requests[i].done([]platform.SpellingRange{{Location: 0, Length: 3}}, nil)
	}
	wg.Wait()
	onMain(func() {
		if len(s.pending) != 0 {
			t.Errorf("pending=%d", len(s.pending))
		}
	})
}

func TestSpellCheckerCancelWhenMainBusy(t *testing.T) {
	for _, phase := range []string{"queued", "submitted"} {
		t.Run(phase, func(t *testing.T) {
			s, n := newTestSpellChecker(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			start := func() { go func() { _, err := s.Check(ctx, "speling", ""); done <- err }() }
			if phase == "submitted" {
				start()
				request := wait.For(t, "submitted native check", n.requests)
				t.Cleanup(func() { request.done(nil, nil) })
			}
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			postMain(func() { close(entered); <-release })
			wait.For(t, "occupied main thread", entered)
			if phase == "queued" {
				start()
				wait.Until(t, "request queued behind occupied main thread", func() bool {
					loop.mu.Lock()
					defer loop.mu.Unlock()
					return len(loop.queue) != 0
				})
			}
			cancel()
			// Returning while release still blocks the UI models a
			// quit listener canceling and joining its worker.
			if err := wait.For(t, "cancel without UI progress", done); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			unblock()
			onMain(func() {})
			if len(n.requests) != 0 {
				t.Fatal("canceled queued request reached native checker")
			}
		})
	}
}

func TestSpellCheckerCanceledQuitKeepsSession(t *testing.T) {
	for _, phase := range []string{"before", "will"} {
		t.Run(phase, func(t *testing.T) {
			s, n := newTestSpellChecker(t)
			attempted := make(chan struct{})
			prevent := func(e *QuitEvent) { e.PreventDefault(); close(attempted) }
			var off func()
			if phase == "before" {
				off = App.OnBeforeQuit(prevent)
			} else {
				off = App.OnWillQuit(prevent)
			}
			defer off()
			done := make(chan error, 1)
			go func() { _, err := s.Check(context.Background(), "speling", ""); done <- err }()
			request := wait.For(t, "native check before canceled quit", n.requests)
			App.Quit()
			wait.For(t, "canceled quit callback", attempted)
			onMain(func() {})
			request.done(nil, nil)
			if err := wait.For(t, "check after canceled quit", done); err != nil {
				t.Fatal(err)
			}
			if err := s.Ignore("word"); err != nil {
				t.Fatal(err)
			}
			onMain(func() {
				if n.closed != 0 {
					t.Error("canceled quit closed native session")
				}
			})
		})
	}
}

func TestSpellCheckerQuitHelper(t *testing.T) {
	if os.Getenv("MYGO_SPELL_QUIT_HELPER") != "1" {
		return
	}
	s, n := newTestSpellChecker(t)
	postMain(func() {
		_, err := s.Check(context.Background(), "speling", "")
		if errors.Is(err, ErrSpellCheckerClosed) {
			fmt.Println("SPELL_QUIT_UNBLOCKED")
		} else {
			fmt.Printf("SPELL_QUIT_ERROR %v\n", err)
		}
	})
	wait.For(t, "check waiting in main event pump", n.requests)
	// The fake backend deliberately never completes this native request.
	// The accepted quit phase must release the nested Check before Run exits.
	App.Quit()
	wait.For(t, "helper process exit", make(chan struct{}))
}

func TestSpellCheckerQuitUnblocksMainCheck(t *testing.T) {
	cmd := exec.CommandContext(wait.Context(t), os.Args[0], "-test.run=^TestSpellCheckerQuitHelper$")
	cmd.Env = append(os.Environ(), "MYGO_SPELL_QUIT_HELPER=1")
	output, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte("SPELL_QUIT_UNBLOCKED")) {
		t.Fatalf("quit helper: %v\n%s", err, output)
	}
}
