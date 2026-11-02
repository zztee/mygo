package richtext

import (
	"github.com/egoist/mygo/ui"
	"strings"
	"testing"
)

func TestBaselineOffsetTagStyleEditing(t *testing.T) {
	source := TextOf(Paragraph{Runs: []Run{
		{Text: "Water H"}, {Text: "2", Attributes: Attributes{Size: 16, Tag: "lower"}},
		{Text: "O and x"}, {Text: "2", Attributes: Attributes{Size: 16, Tag: "raise"}},
		{Text: ". " + strings.Repeat("long text wraps ", 15)},
	}})
	doc := NewDocument(source)
	original, md, version := doc.Text(), doc.Text().Markdown(), doc.Version()
	var scroll Scroll
	tt := baseTester(doc, EditorOptions{Scroll: &scroll, Label: "Inline document", Font: ui.Font{Size: 24}, TagStyle: func(tag string, s *ui.Span) {
		switch tag {
		case "raise":
			s.BaselineOffset = 8
		case "lower":
			s.BaselineOffset = -6
		}
	}})
	if a, ok := tt.Access("Inline document", ui.RoleAuto); !ok || a.Value != source.String() {
		t.Fatalf("accessibility %+v, found=%v", a, ok)
	}
	if scroll.Content() < 60 {
		t.Fatal("long inline text did not wrap")
	}
	tt.Press(10, scroll.TextY(0)+8)
	tt.Release(10, scroll.TextY(0)+8)
	doc.SetSelection(7, 8)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyC)
	if tt.Clipboard() != "2" {
		t.Fatalf("copy %q", tt.Clipboard())
	}
	if !doc.Text().Same(original) || doc.Version() != version || doc.Text().Markdown() != md {
		t.Fatal("display or copy changed the document")
	}
	doc.SetSelection(7, 7)
	tt.Frame()
	tt.Key(ui.Shift, ui.KeyRight)
	if a, c := doc.Selection(); a != 7 || c != 8 {
		t.Fatalf("keyboard selection %d..%d", a, c)
	}
	tt.Type("3")
	tt.Frame()
	if !strings.HasPrefix(doc.Text().String(), "Water H3O") {
		t.Fatalf("typed %q", doc.Text().String())
	}
	doc.Undo()
	tt.Frame()
	if !doc.Text().Equal(original) || doc.CanUndo() {
		t.Fatal("editing a shifted span did not undo once")
	}
}
