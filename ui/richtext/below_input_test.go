package richtext

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// belowPressTester puts a preview under a code block in a narrow column.
func belowPressTester(t *testing.T) (*ui.Tester, *Document, *Scroll, *below, *EditorOptions, *[]pressed) {
	t.Helper()
	d := NewDocument(ParseMarkdown("one\n\n```go\nx := 1\n```\n\n" + strings.Repeat("tail\n\n", 30)))
	d.SetSelection(0, 0)
	s, b := &Scroll{}, &below{at: 4, h: 80}
	seen := &[]pressed{}
	opts := &EditorOptions{Scroll: s, MaxWidth: 240, Margin: [4]float32{12, 20, 16, 30}, Below: b.fn}
	opts.OnBelow = func(p int, x, y float32, ev ui.InputEvent) bool {
		*seen = append(*seen, pressed{p, x, y, ev.Button})
		return true
	}
	tt := ui.NewTester(func(c *ui.Context) { EditorBase(c, d, *opts).Fill() }, 500, 240)
	tt.Frame()
	tt.Frame()
	return tt, d, s, b, opts, seen
}

func TestBelowPress(t *testing.T) {
	tt, d, _, b, opts, seen := belowPressTester(t)
	original, version := d.Text().String(), d.Version()
	r := b.drawn[len(b.drawn)-1]
	tt.ClickAt(r.X+17, r.Y+23)
	if len(*seen) != 1 || (*seen)[0].i != 1 || !near((*seen)[0].x, 17) || !near((*seen)[0].y, 23) || (*seen)[0].button != 0 {
		t.Fatalf("presses %+v, rectangle %+v", *seen, r)
	}
	if a, c := d.Selection(); a != 0 || c != 0 {
		t.Fatalf("consumed press moved selection to %d-%d", a, c)
	}
	if d.Text().String() != original || d.Version() != version {
		t.Fatal("preview press edited the document")
	}
	opts.OnBelow = func(p int, x, y float32, ev ui.InputEvent) bool { return false }
	tt.Frame()
	tt.Advance(time.Second)
	tt.ClickAt(r.X+17, r.Y+23)
	if a, c := d.Selection(); a != c || c < 4 || c > 10 {
		t.Fatalf("unconsumed press did not place the caret on the block: %d-%d", a, c)
	}
}

func TestBelowContextMenu(t *testing.T) {
	tt, d, _, b, opts, seen := belowPressTester(t)
	d.SetSelection(0, 3)
	chosen := 0
	opts.ContextMenu = func(m *ui.Menu, attachment int) {
		if len(*seen) != 1 || (*seen)[0].button != 1 || attachment != -1 {
			t.Errorf("menu before press or on an attachment: %+v, %d", *seen, attachment)
		}
		if m.Item("Preview").Chosen() {
			chosen++
		}
	}
	tt.Frame()
	r := b.drawn[len(b.drawn)-1]
	tt.RightClickAt(r.X+19, r.Y+25)
	if menu := tt.Menu(); !slices.Contains(menu, "Preview") {
		t.Fatalf("preview menu missing: %q", menu)
	}
	if err := tt.ChooseMenuItem("Preview"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if chosen != 1 || len(*seen) != 1 {
		t.Fatalf("chosen %d, presses %+v", chosen, *seen)
	}
	if a, c := d.Selection(); a != 0 || c != 3 {
		t.Fatalf("secondary press moved selection to %d-%d", a, c)
	}
}

func TestBelowPressBoundsAndChanges(t *testing.T) {
	tt, _, s, b, opts, seen := belowPressTester(t)
	r := b.drawn[len(b.drawn)-1]
	// Code padding, paragraph gaps, column margins and the scrollbar
	// are outside the preview, even though hit testing finds its paragraph.
	for _, pt := range [][2]float32{
		{r.X + 10, r.Y - 1}, {r.X + 10, r.Y + r.H},
		{r.X - 1, r.Y + 10}, {r.X + r.W, r.Y + 10}, {499, r.Y + 10},
	} {
		tt.RightClickAt(pt[0], pt[1])
		tt.CloseMenu()
	}
	if len(*seen) != 0 {
		t.Fatalf("outside presses %+v", *seen)
	}
	opts.ReadOnly = true
	tt.Frame()
	tt.ClickAt(r.X+10, r.Y+10)
	if len(*seen) != 1 {
		t.Fatalf("read-only preview presses %+v", *seen)
	}
	b.h = 0
	tt.Frame()
	tt.RightClickAt(r.X+10, r.Y+10)
	tt.CloseMenu()
	if len(*seen) != 1 {
		t.Fatalf("removed preview still takes presses %+v", *seen)
	}
	// Removing Below must discard its old room.
	opts.Below = nil
	s.ScrollTo(0)
	tt.Frame()
	tt.ClickAt(r.X+10, r.Y+10)
	if len(*seen) != 1 {
		t.Fatalf("disabled preview still takes presses %+v", *seen)
	}
}

func TestBelowPressScrolledCoordinates(t *testing.T) {
	tt, d, s, b, _, seen := belowPressTester(t)
	version := d.Version()
	s.ScrollTo(30)
	tt.Frame()
	if s.Offset() != 30 {
		t.Fatalf("scroll offset %v", s.Offset())
	}
	r := b.drawn[len(b.drawn)-1]
	tt.RightClickAt(r.X+31, r.Y+35)
	tt.CloseMenu()
	if len(*seen) != 1 || (*seen)[0].i != 1 || !near((*seen)[0].x, 31) || !near((*seen)[0].y, 35) {
		t.Fatalf("scrolled presses %+v, rectangle %+v", *seen, r)
	}
	// A wheel over the preview remains the editor's.
	tt.Scroll(r.X+20, r.Y+20, 0, 20)
	if s.Offset() != 50 {
		t.Fatalf("wheel over preview did not scroll: %v", s.Offset())
	}
	if len(*seen) != 1 || d.Version() != version {
		t.Fatalf("scroll generated presses/edits: %+v, %d", *seen, d.Version())
	}
}
