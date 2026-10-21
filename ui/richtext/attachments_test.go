package richtext

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// pressed is a press OnAttachment saw.
type pressed struct {
	i      int
	x, y   float32
	button int
}

// attachmentTester builds an editor of "ab", an attachment w by h and
// "cd", whose OnAttachment records the presses it sees and takes them
// when take is set, and whose context menu offers "Cell" when the press
// was on the attachment.
func attachmentTester(t *testing.T, w, h float32, take *bool) (*ui.Tester, *Document, *Scroll, *[]pressed, *int) {
	t.Helper()
	at := &Attachment{Name: "box", Width: w, Height: h, Draw: func(*ui.Painter, ui.Rect) {}}
	doc := NewDocument(TextOf(Paragraph{Runs: []Run{{Text: "ab"}, {Text: string(ObjectReplacement), Attributes: Attributes{Attachment: at}}, {Text: "cd"}}}))
	scroll := &Scroll{}
	var seen []pressed
	chosen := new(int)
	tt := ui.NewTester(func(c *ui.Context) {
		EditorBase(c, doc, EditorOptions{
			Scroll: scroll,
			OnAttachment: func(i int, x, y float32, ev ui.InputEvent) bool {
				seen = append(seen, pressed{i, x, y, ev.Button})
				return *take
			},
			ContextMenu: func(m *ui.Menu, attachment int) {
				if attachment != 2 {
					return
				}
				if m.Item("Cell").Chosen() {
					*chosen++
				}
			},
		}).Fill()
	}, 500, 400)
	tt.Frame()
	return tt, doc, scroll, &seen, chosen
}

func TestAttachmentPress(t *testing.T) {
	take := true
	tt, doc, scroll, seen, _ := attachmentTester(t, 100, 50, &take)
	box, ok := scroll.AttachmentBox(2)
	if !ok || box.W != 100 || box.H != 50 {
		t.Fatalf("box %+v %v", box, ok)
	}
	if _, ok := scroll.AttachmentBox(1); ok {
		t.Fatal("a letter has a box")
	}
	if _, ok := scroll.AttachmentBox(99); ok {
		t.Fatal("past the text has a box")
	}
	if i, ok := scroll.IndexAt(box.X+box.W/2, box.Y+box.H/2); !ok || i < 2 || i > 3 {
		t.Fatalf("the box is not where the attachment is: %d", i)
	}

	doc.SetSelection(0, 0)
	tt.Frame()
	tt.ClickAt(box.X+10, box.Y+20)
	if want := []pressed{{2, 10, 20, 0}}; !slices.Equal(*seen, want) {
		t.Fatalf("presses %+v, want %+v", *seen, want)
	}
	if a, c := doc.Selection(); a != 0 || c != 0 {
		t.Fatalf("a press taken moved the selection to %d-%d", a, c)
	}

	// Beside the box, the press is the editor's.
	tt.ClickAt(box.X+box.W+30, box.Y+box.H/2)
	if len(*seen) != 1 {
		t.Fatalf("a press beside the attachment reached it: %+v", *seen)
	}

	// Not taken, it places the caret as before.
	take = false
	tt.ClickAt(box.X+box.W-5, box.Y+box.H/2)
	if a, c := doc.Selection(); len(*seen) != 2 || a != c || c != 3 {
		t.Fatalf("presses %+v, selection %d-%d", *seen, a, c)
	}
}

// A box shrunk to a narrow column gives the point in the attachment's own
// size.
func TestAttachmentPressScaled(t *testing.T) {
	take := true
	tt, _, scroll, seen, _ := attachmentTester(t, 2000, 400, &take)
	box, ok := scroll.AttachmentBox(2)
	if !ok || box.W > 500 || box.W < 100 {
		t.Fatalf("box %+v", box)
	}
	s := box.W / 2000
	if d := box.H - 400*s; d > 0.01 || d < -0.01 {
		t.Fatalf("box %+v does not keep the ratio", box)
	}
	tt.ClickAt(box.X+box.W/2, box.Y+box.H/4)
	if len(*seen) != 1 {
		t.Fatalf("presses %+v", *seen)
	}
	p := (*seen)[0]
	if p.x < 999 || p.x > 1001 || p.y < 99 || p.y > 101 {
		t.Fatalf("pressed at %v,%v, want 1000,100", p.x, p.y)
	}
}

// The secondary button tells the app before the context menu opens,
// whose items the app adds.
func TestAttachmentContextMenu(t *testing.T) {
	take := true
	tt, doc, scroll, seen, chosen := attachmentTester(t, 100, 50, &take)
	box, _ := scroll.AttachmentBox(2)
	doc.SetSelection(0, 0)
	tt.Frame()
	tt.RightClickAt(box.X+30, box.Y+10)
	if want := []pressed{{2, 30, 10, 1}}; !slices.Equal(*seen, want) {
		t.Fatalf("presses %+v, want %+v", *seen, want)
	}
	if got := tt.Menu(); !slices.Contains(got, "Cell") {
		t.Fatalf("menu %q", got)
	}
	if err := tt.ChooseMenuItem("Cell"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if *chosen != 1 {
		t.Fatalf("chosen %d times", *chosen)
	}
	if a, c := doc.Selection(); a != 0 || c != 0 {
		t.Fatalf("the menu moved the selection to %d-%d", a, c)
	}

	// On the text, the menu is not the attachment's.
	tt.RightClickAt(box.X+box.W+30, box.Y+box.H/2)
	if got := tt.Menu(); slices.Contains(got, "Cell") || len(*seen) != 1 {
		t.Fatalf("menu %q, presses %+v", got, *seen)
	}
	tt.CloseMenu()
	// Nor from the keyboard after it.
	tt.RightClickAt(box.X+30, box.Y+10)
	tt.CloseMenu()
	tt.Key(0, ui.KeyRight)
	tt.Key(ui.Shift, ui.KeyF10)
	if got := tt.Menu(); slices.Contains(got, "Cell") {
		t.Fatalf("menu from the keyboard %q", got)
	}
}
