package richtext

import (
	"bytes"
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

func TestCodeNumbersToggleAndCopy(t *testing.T) {
	source := "Intro\n\n~~~go {title=sample}\nfirst\nsecond\n~~~\n\nEnd"
	doc := NewDocument(ParseMarkdown(source))
	original, markdown, version := doc.Text(), doc.Text().Markdown(), doc.Version()
	var s Scroll
	opts := EditorOptions{Scroll: &s, Label: "Code document"}
	tt := ui.NewTester(func(c *ui.Context) { Editor(c, doc, opts).Fill() }, 500, 400)
	tt.Frame()
	first := strings.Index(original.String(), "first")
	p := original.paraAt(first)
	x, _ := s.v.origin(p)
	opts.CodeLineNumbers = true
	tt.Frame()
	numbered, _ := s.v.origin(p)
	if numbered <= x || s.v.paras[p].codeNumber != 1 || s.v.paras[p+1].codeNumber != 2 {
		t.Fatalf("numbered x=%v, original=%v; labels %d, %d", numbered, x, s.v.paras[p].codeNumber, s.v.paras[p+1].codeNumber)
	}
	if got, ok := tt.Access("Code document", ui.RoleAuto); !ok || got.Value != original.String() {
		t.Fatalf("accessibility value: %+v, found=%v", got, ok)
	}
	tt.Press(numbered+2, s.TextY(first)+5)
	tt.Release(numbered+2, s.TextY(first)+5)
	doc.SetSelection(first, first+len("first\nsecond"))
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyC)
	if got := tt.Clipboard(); got != "first\nsecond" {
		t.Fatalf("copied %q", got)
	}
	data, err := tt.ClipboardData().Read(transfer.HTML)
	if err != nil || strings.Contains(string(data), ">1<") {
		t.Fatalf("HTML clipboard %s, %v", data, err)
	}
	if !original.Same(doc.Text()) || doc.Text().Markdown() != markdown || doc.Version() != version {
		t.Fatal("displaying or copying line numbers changed the document")
	}
	doc.SetSelection(0, 0)
	opts.CodeLineNumbers = false
	tt.Frame()
	restored, _ := s.v.origin(p)
	if !near(restored, x) {
		t.Fatalf("disabled origin %v, want %v", restored, x)
	}
	// A fresh unfocused view has the exact pixels of the default, disabled API.
	plain := baseTester(NewDocument(original), EditorOptions{})
	explicit := baseTester(NewDocument(original), EditorOptions{CodeLineNumbers: false})
	if !bytes.Equal(plain.Image().Pix, explicit.Image().Pix) {
		t.Fatal("the disabled option changed the rendering")
	}
	doc.Undo()
	if !original.Same(doc.Text()) {
		t.Fatal("the display option created an undo step")
	}
}

func TestCodeNumbersWrapAndGutterHit(t *testing.T) {
	long := strings.Repeat("wide code words ", 18)
	doc := NewDocument(TextOf(
		Paragraph{Style: ParagraphStyle{CodeBlock: true}, Runs: []Run{{Text: long}}},
		Paragraph{Style: ParagraphStyle{CodeBlock: true}, Runs: []Run{{Text: "next"}}},
	))
	var s Scroll
	tt := ui.NewTester(func(c *ui.Context) {
		Editor(c, doc, EditorOptions{CodeLineNumbers: true, Scroll: &s}).Fill()
	}, 260, 700)
	tt.Frame()
	v := s.v
	pv := v.paraLayout(0)
	first := pv.layout.CaretAt(ui.TextCaretPosition{})
	last := pv.layout.CaretAt(ui.TextCaretPosition{Index: pv.units[len(pv.units)-1]})
	if last.Y <= first.Y || pv.codeNumber != 1 || v.paras[1].codeNumber != 2 {
		t.Fatalf("wrapped lines first=%+v last=%+v, numbers %d/%d", first, last, pv.codeNumber, v.paras[1].codeNumber)
	}
	// The gutter has ink only beside the first visual line, not its continuation.
	x, y := v.origin(0)
	x += v.rect.X + v.padX
	y += float64(v.rect.Y+v.padY) - v.scroll
	image := tt.Image()
	ink := func(top float32) bool {
		background := image.RGBAAt(int(x-v.codeGutter), int(top+first.H/2))
		for yy := int(top); yy < int(top+first.H); yy++ {
			for xx := int(x - v.codeGutter); xx < int(x-v.paraFont(v.para(0).style).Size*0.75); xx++ {
				if image.RGBAAt(xx, yy) != background {
					return true
				}
			}
		}
		return false
	}
	if !ink(float32(y)+first.Y) || ink(float32(y)+last.Y) {
		t.Fatalf("number ink first=%v last=%v; origin=%v/%v, carets=%+v/%+v gutter=%v", ink(float32(y)+first.Y), ink(float32(y)+last.Y), x, y, first, last, v.codeGutter)
	}
	start := doc.Text().ParagraphStart(1)
	gx, gy := v.origin(1)
	gx += v.rect.X + v.padX
	gy += float64(v.rect.Y+v.padY) - v.scroll
	gx -= v.codeGutter / 2
	tt.Press(gx, float32(gy)+first.H/2)
	tt.Release(gx, float32(gy)+first.H/2)
	if a, z := doc.Selection(); a != start || z != start {
		t.Fatalf("gutter click selected %d–%d, want %d", a, z, start)
	}
	tt.Press(x-v.codeGutter/2, float32(y)+first.H/2)
	tt.Move(gx, float32(gy)+first.H/2)
	tt.Release(gx, float32(gy)+first.H/2)
	tt.Key(ui.Cmd, ui.KeyC)
	if got := tt.Clipboard(); got != long+"\n" {
		t.Fatalf("gutter drag copied %q", got)
	}
}

func TestCodeNumbersRenumberAndEmpty(t *testing.T) {
	doc := NewDocument(ParseMarkdown("```go\na\nb\n```\n\n~~~text\n\n~~~"))
	var s Scroll
	tt := baseTester(doc, EditorOptions{CodeLineNumbers: true, Scroll: &s})
	check := func(want ...int) {
		t.Helper()
		tt.Frame()
		if doc.Text().ParagraphCount() != len(want) {
			t.Fatalf("paragraph count %d, want %d", doc.Text().ParagraphCount(), len(want))
		}
		for p, n := range want {
			if got := s.v.codeNumber(p); got != n {
				t.Fatalf("paragraph %d number %d, want %d", p, got, n)
			}
		}
	}
	check(1, 2, 1)
	doc.SetSelection(0, 0)
	doc.InsertText("inserted\n")
	check(1, 2, 3, 1)
	doc.Undo()
	check(1, 2, 1)
	doc.Redo()
	check(1, 2, 3, 1)
	doc.SetSelection(0, 0)
	doc.FormatParagraphs(func(st *ParagraphStyle) { st.CodeBlock = false })
	check(0, 1, 2, 1)
	doc.SetText(NewText(""))
	check(0)
	if s.v.paras[0].codeLabel != nil {
		t.Fatal("an empty document has a code number")
	}
	doc.SetText(TextOf(Paragraph{Style: ParagraphStyle{CodeBlock: true}}))
	check(1)
	if s.v.paras[0].codeLabel == nil || s.v.paras[0].codeNumber != 1 {
		t.Fatal("an empty code block has no first line number")
	}
}

func TestCodeNumbersCacheAndDigits(t *testing.T) {
	doc := NewDocument(codeNumberText(99))
	var s Scroll
	tt := baseTester(doc, EditorOptions{CodeLineNumbers: true, Scroll: &s})
	v := s.v
	if got := v.codeNumber(98); got != 99 {
		t.Fatalf("last number %d", got)
	}
	prefix := &v.codeNumbers[0]
	gutter := v.codeGutter
	label := v.paras[0].codeLabel
	tt.Frame()
	if &v.codeNumbers[0] != prefix || v.paras[0].codeLabel != label {
		t.Fatal("idle frame rebuilt the number cache")
	}
	end := doc.Text().Len()
	doc.SetSelection(end, end)
	doc.InsertText("\nnew")
	tt.Frame()
	if v.codeGutter <= gutter || v.codeNumber(99) != 100 {
		t.Fatalf("gutter %v -> %v, number %d", gutter, v.codeGutter, v.codeNumber(99))
	}
	if &v.codeNumbers[0] != prefix {
		t.Fatal("an edit at the end discarded the cached prefix")
	}
	doc.Undo()
	tt.Frame()
	if !near(v.codeGutter, gutter) {
		t.Fatalf("undo gutter %v, want %v", v.codeGutter, gutter)
	}
}
