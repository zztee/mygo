package ui

import "testing"

// Batch delivers events immediately but leaves frame construction to
// the outermost return, even when nested code explicitly asks for one.
func TestTesterBatchDefersFrames(t *testing.T) {
	frames := 0
	text := ""
	tt := NewTester(func(c *Context) {
		frames++
		TextInput(c, &text).Label("Text").Fill()
	}, 400, 200)
	if err := tt.Click("Text"); err != nil {
		t.Fatal(err)
	}
	defer tt.rt.close()
	before := frames
	tt.Batch(func() {
		tt.Type("a")
		tt.Batch(func() { tt.Type("b"); tt.Frame() })
		tt.Type("c")
		if frames != before {
			t.Fatalf("built %d frames inside batch", frames-before)
		}
	})
	if frames <= before || text != "abc" {
		t.Fatalf("frames %d -> %d, text %q", before, frames, text)
	}
	tt.Type("d")
	if text != "abcd" {
		t.Fatalf("input after batch %q", text)
	}
}

// A panic in the supplied function does not leave automatic frames off.
func TestTesterBatchRestoresAfterPanic(t *testing.T) {
	frames := 0
	tt := NewTester(func(c *Context) { frames++; Text(c, "body") }, 200, 100)
	defer tt.rt.close()
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic was swallowed")
			}
		}()
		tt.Batch(func() { panic("batch") })
	}()
	before := frames
	tt.Frame()
	if frames <= before {
		t.Fatal("frames stayed suspended")
	}
}
