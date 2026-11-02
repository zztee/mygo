//go:build snapshot

package richtext

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func baselineSnapshotText() Text {
	return TextOf(Paragraph{Runs: []Run{
		{Text: "Native inline text ", Attributes: Attributes{Size: 24}},
		{Text: "raised", Attributes: Attributes{Size: 17, Tag: "raise", Underline: true, Background: ui.Hex("#f4df79")}},
		{Text: " and ", Attributes: Attributes{Size: 24}},
		{Text: "lowered", Attributes: Attributes{Size: 17, Tag: "lower", Strikethrough: true}},
		{Text: ". " + strings.Repeat("Wrapping stays in the text. ", 8), Attributes: Attributes{Size: 24}},
	}}, Paragraph{Runs: []Run{
		{Text: "Water H", Attributes: Attributes{Size: 24}},
		{Text: "2", Attributes: Attributes{Size: 17, Tag: "lower"}},
		{Text: "O and x", Attributes: Attributes{Size: 24}},
		{Text: "2", Attributes: Attributes{Size: 17, Tag: "raise"}},
		{Text: ".", Attributes: Attributes{Size: 24}},
	}})
}

func baselineSnapshots(t *testing.T, name string, style func(string, *ui.Span)) {
	t.Helper()
	dir := os.Getenv("SNAP_DIR")
	if dir == "" {
		t.Skip("SNAP_DIR is not set")
	}
	for _, dark := range []bool{false, true} {
		suffix, theme := "light", ui.LightTheme()
		if dark {
			suffix, theme = "dark", ui.DarkTheme()
		}
		doc := NewDocument(baselineSnapshotText())
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme)
			Editor(c, doc, EditorOptions{TagStyle: style, Font: ui.Font{Size: 24}, Margin: [4]float32{24, 24, 24, 24}}).Fill()
		}, 540, 480)
		tt.Frame()
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, name+"-"+suffix+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("image: %v; close: %v", err, closeErr)
		}
	}
}

func TestBaselineZeroSnapshots(t *testing.T) { baselineSnapshots(t, "zero", nil) }
