//go:build snapshot

package richtext

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUnderlinesSnapshots(t *testing.T) {
	dir := os.Getenv("SNAP_DIR")
	if dir == "" {
		t.Skip("SNAP_DIR is not set")
	}
	for _, dark := range []bool{false, true} {
		for _, on := range []bool{false, true} {
			name := "light"
			if dark {
				name = "dark"
			}
			if on {
				name += "-marked"
			} else {
				name += "-plain"
			}
			d := NewDocument(NewText("🙂 拼写 Speling mistakes\nA long wrng phrase that wraps across several lines."))
			tt := ui.NewTester(func(c *ui.Context) {
				Editor(c, d, EditorOptions{Font: ui.Font{Size: 24}, MaxWidth: 280, Underlines: func(text Text, paragraph int) []Underline {
					if !on {
						return nil
					}
					return []Underline{{Start: 5, End: 12, Wavy: true, Color: ui.RGB(230, 50, 50)}, {Start: 24, End: text.Len(), Wavy: true, Color: ui.RGB(230, 50, 50)}}
				}}).Fill()
			}, 380, 280)
			tt.SetDark(dark)
			tt.Frame()
			f, err := os.Create(filepath.Join(dir, name+".png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatalf("snapshot: %v, close: %v", err, closeErr)
			}
		}
	}
}
