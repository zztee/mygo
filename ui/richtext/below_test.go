package richtext

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// below is an EditorOptions.Below adding room of h under the paragraph
// starting at rune at, which records where it was drawn.
type below struct {
	at    int
	h     float32
	calls int
	drawn []ui.Rect
}

func (b *below) fn(t Text, p int, st ParagraphStyle) (float32, func(*ui.Painter, ui.Rect)) {
	b.calls++
	if t.ParagraphStart(p) != b.at || b.h == 0 {
		return 0, nil
	}
	return b.h, func(p *ui.Painter, r ui.Rect) { b.drawn = append(b.drawn, r) }
}

// Below adds room under a paragraph, after its block of code, which the
// paragraphs after it move down for, and draws in it.
func TestBelowAddsRoomUnderAParagraph(t *testing.T) {
	doc := NewDocument(ParseMarkdown("one\n\n```go\nx := 1\ny := 2\n```\n\nthree"))
	doc.SetSelection(0, 0)
	var plain Scroll
	baseTester(doc, EditorOptions{Scroll: &plain})
	// The block's last line, y := 2, starts at rune 11.
	b := &below{at: 11, h: 40}
	var s Scroll
	tt := baseTester(doc, EditorOptions{Scroll: &s, Below: b.fn})
	if d := s.TextY(18) - plain.TextY(18); !near(d, 40) {
		t.Fatalf("the paragraph after moved down %v, not 40", d)
	}
	if d := s.TextY(11) - plain.TextY(11); !near(d, 0) {
		t.Fatalf("the line it is under moved %v", d)
	}
	if !near(s.Content()-plain.Content(), 40) {
		t.Fatalf("the text grew %v", s.Content()-plain.Content())
	}
	if len(b.drawn) == 0 {
		t.Fatal("not drawn")
	}
	r := b.drawn[len(b.drawn)-1]
	x, w := s.Column()
	if !near(r.H, 40) || !near(r.X, x) || !near(r.W, w) {
		t.Fatalf("drawn in %+v, column at %v, %v wide", r, x, w)
	}
	// Under the block: below its last line and its padding, above the
	// next paragraph.
	v := s.v
	_, lineTop, lh := v.caretAt(11, 0, ui.TextDownstream)
	if bottom := float32(lineTop) + lh - float32(v.scroll) + v.padY; r.Y < bottom+v.look.size*v.look.CodeBlockPadding[1]-0.5 {
		t.Fatalf("drawn at %v, over the block ending at %v", r.Y, bottom)
	}
	if next := s.ParaTop(18); r.Y+r.H > next+0.5 {
		t.Fatalf("drawn down to %v, past the next paragraph at %v", r.Y+r.H, next)
	}

	// The pointer in the room goes to the line above it.
	tt.Press(5, r.Y+r.H/2)
	tt.Release(5, r.Y+r.H/2)
	if _, c := doc.Selection(); c < 11 || c > 17 {
		t.Fatalf("a click under the block put the caret at %d", c)
	}
	// Down from that line goes past the room, to the next paragraph.
	doc.SetSelection(12, 12)
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	if _, c := doc.Selection(); c < 18 {
		t.Fatalf("down from the block's last line went to %d", c)
	}
	tt.Key(0, ui.KeyUp)
	if _, c := doc.Selection(); c < 11 || c > 17 {
		t.Fatalf("up from the paragraph after went to %d", c)
	}

	// Its height follows what Below says, the text unchanged.
	b.h = 10
	tt.Frame()
	if d := s.TextY(18) - plain.TextY(18); !near(d, 10) {
		t.Fatalf("the room of 10 moved the paragraph after %v", d)
	}
	b.h = 0
	tt.Frame()
	if d := s.TextY(18) - plain.TextY(18); !near(d, 0) {
		t.Fatalf("no room moved the paragraph after %v", d)
	}
}

// ParagraphStart is where a paragraph starts by its number.
func TestParagraphStart(t *testing.T) {
	text := NewText("one\ntwo\n\nfour")
	for p, want := range []int{0, 4, 8, 9, 9} {
		if got := text.ParagraphStart(p); got != want {
			t.Fatalf("paragraph %d starts at %d, want %d", p, got, want)
		}
	}
}

// Below is asked about the paragraphs in view once a frame, not those out
// of it.
func TestBelowAsksOnceAFrame(t *testing.T) {
	text := ""
	for i := range 2000 {
		if i > 0 {
			text += "\n"
		}
		text += "a line of the text"
	}
	doc := NewDocument(NewText(text))
	doc.SetSelection(0, 0)
	b := &below{at: -1}
	var s Scroll
	tt := baseTester(doc, EditorOptions{Scroll: &s, Below: b.fn})
	shown := s.v.last - s.v.first + 1
	b.calls = 0
	tt.Frame()
	if b.calls == 0 || b.calls > shown+2 {
		t.Fatalf("%d calls for %d paragraphs in view", b.calls, shown)
	}
}
