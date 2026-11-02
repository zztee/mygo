//go:build snapshot

package richtext

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCodeNumbersSnapshots(t *testing.T) {
	dir := os.Getenv("SNAP_DIR")
	if dir == "" {
		t.Skip("SNAP_DIR is not set")
	}
	for _, on := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			name, theme := "light", ui.LightTheme()
			if dark {
				name, theme = "dark", ui.DarkTheme()
			}
			doc := NewDocument(ParseMarkdown("# Code blocks\n\n```go\nfunc greet() {\n    println(\"A long code line wraps in this narrow editor, keeping a single logical number.\")\n}\n```\n\nEmpty block:\n\n```text\n\n```\n\n```python\nprint(\"Another block starts at one\")\n```"))
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme)
				Editor(c, doc, EditorOptions{CodeLineNumbers: on, Margin: [4]float32{18, 18, 18, 18}}).Fill()
			}, 540, 520)
			tt.Frame()
			tt.Frame()
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%t.png", name, on)))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				t.Fatalf("encode: %v; close: %v", err, closeErr)
			}
		}
	}
}
