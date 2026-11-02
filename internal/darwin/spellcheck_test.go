//go:build darwin

package darwin

import (
	"testing"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

func TestSpellCheckerNativeMessages(t *testing.T) {
	load()
	api := spellingMessages()
	withPool(func() {
		// Validate the two-register NSRange ABI without using the user's
		// spelling service or changing their learned dictionary.
		var valueWithRange func(id, objc.SEL, nsRange) id
		purego.RegisterFunc(&valueWithRange, msgSendAddr)
		want := nsRange{Location: 3, Length: 7}
		value := valueWithRange(class("NSValue"), sel("valueWithRange:"), want)
		if got := api.rangeOf(value, sel("rangeValue")); got != want {
			t.Fatalf("range=%v, want %v", got, want)
		}
		key := constString(libAppKit, "NSTextCheckingOrthographyKey")
		if key == 0 || goString(key) == "" {
			t.Fatal("orthography option key missing")
		}
		options := spellingOptions("en_GB")
		orthography := send(options, "objectForKey:", uintptr(key))
		if got := goString(send(orthography, "dominantLanguage")); got != "en-GB" {
			t.Fatalf("language=%q", got)
		}
		if spellingOptions("") != 0 {
			t.Fatal("default language overrides system preference")
		}
		// NewBlock reuses one callback per signature, including for repeated
		// checks; Cocoa copies and releases each request's closure itself.
		calls := 0
		for range 3 {
			block := newBlock(func(_ objc.Block, _ int, _ id, _ id, _ int) { calls++ })
			copy := block.Copy()
			block.Release()
			callBlock(uintptr(copy), 1, 0, 0, 0)
			copy.Release()
		}
		if calls != 3 {
			t.Fatalf("callbacks=%d", calls)
		}
	})
}
