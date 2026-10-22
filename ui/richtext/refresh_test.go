package richtext

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// refreshTester builds an editor of 80 paragraphs, paragraph 20 holding
// an attachment 100 by 50 after a word, and shows paragraph 25 at the
// top, paragraph 20 laid out on the way.
func refreshTester(t *testing.T) (*ui.Tester, *Document, *Scroll, *Attachment, int) {
	t.Helper()
	at := &Attachment{Name: "picture", Width: 100, Height: 50, Draw: func(*ui.Painter, ui.Rect) {}}
	var paras []Paragraph
	for i := range 80 {
		p := Paragraph{Runs: []Run{{Text: fmt.Sprintf("Paragraph %d.", i)}}}
		if i == 20 {
			p.Runs = append(p.Runs, Run{Text: string(ObjectReplacement), Attributes: Attributes{Attachment: at}})
		}
		paras = append(paras, p)
	}
	doc := NewDocument(TextOf(paras...))
	doc.SetSelection(3, 3)
	scroll := &Scroll{}
	tt := ui.NewTester(func(c *ui.Context) {
		EditorBase(c, doc, EditorOptions{Scroll: scroll}).Fill()
	}, 500, 300)
	tt.Frame()
	text := doc.Text()
	i := text.ParagraphStart(20) + len("Paragraph 20.")
	scroll.ScrollToText(i, 0)
	tt.Frame()
	scroll.ScrollToText(text.ParagraphStart(25), 0)
	tt.Frame()
	if box, ok := scroll.AttachmentBox(i); !ok || box.H != 50 || box.Y >= 0 {
		t.Fatalf("the attachment is boxed %+v, %v", box, ok)
	}
	return tt, doc, scroll, at, i
}

// An attachment changed in place and refreshed takes its new size in its
// paragraph's layout and height at the next frame, out of view too: the
// text in view stays where it is, the other paragraphs keep their
// layouts, and the document its version, history and selection.
func TestRefreshAttachments(t *testing.T) {
	tt, doc, scroll, at, i := refreshTester(t)
	v := scroll.v
	start25 := doc.Text().ParagraphStart(25)
	top, h20, kept := scroll.ParaTop(start25), v.paras[20].h, v.paras[25].layout
	version := doc.Version()
	at.Height = 250
	doc.RefreshAttachments(at)
	tt.Frame()
	if got := v.paras[20].h; got != h20+200 {
		t.Errorf("paragraph 20 is %v high, want %v", got, h20+200)
	}
	if got := scroll.ParaTop(start25); got != top {
		t.Errorf("the text in view moved from %v to %v", top, got)
	}
	if v.paras[25].layout != kept {
		t.Error("a paragraph not refreshed was laid out again")
	}
	if doc.Version() != version || doc.CanUndo() {
		t.Error("a refresh changed the text or made a step of undo")
	}
	if a, c := doc.Selection(); a != 3 || c != 3 {
		t.Errorf("the selection moved to %d-%d", a, c)
	}
	scroll.ScrollToText(i, 0)
	tt.Frame()
	if box, ok := scroll.AttachmentBox(i); !ok || box.H != 250 {
		t.Errorf("the attachment is boxed %+v, %v", box, ok)
	}
	// Refreshing an attachment the text does not hold does nothing.
	doc.RefreshAttachments(&Attachment{Width: 1, Height: 1})
	doc.RefreshAttachments()
	tt.Frame()
}

// A refresh of a range follows the edits made after it, before the
// editor took it.
func TestRefreshAfterEdit(t *testing.T) {
	tt, doc, scroll, at, i := refreshTester(t)
	v := scroll.v
	h20 := v.paras[20].h
	at.Height = 150
	doc.Refresh(i, i+1)
	doc.SetSelection(0, 0)
	doc.InsertText("New.\n")
	tt.Frame()
	if got := v.paras[21].h; got != h20+100 {
		t.Errorf("the refreshed paragraph, now 21, is %v high, want %v", got, h20+100)
	}
	if box, ok := scroll.AttachmentBox(i + len("New.\n")); !ok || box.H != 150 {
		t.Errorf("the attachment is boxed %+v, %v", box, ok)
	}
}

// An editor that missed more refreshes than the document remembers lays
// out every paragraph again.
func TestRefreshFarBehind(t *testing.T) {
	tt, doc, scroll, at, i := refreshTester(t)
	at.Height = 120
	for range 100 {
		doc.Refresh(0, 0)
	}
	doc.RefreshAttachments(at)
	tt.Frame()
	scroll.ScrollToText(i, 0)
	tt.Frame()
	if box, ok := scroll.AttachmentBox(i); !ok || box.H != 120 {
		t.Errorf("the attachment is boxed %+v, %v", box, ok)
	}
	if len(doc.refreshes) > 64 {
		t.Errorf("the document remembers %d refreshes", len(doc.refreshes))
	}
}
