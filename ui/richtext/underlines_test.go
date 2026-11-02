package richtext

import (
	"bytes"
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestUnderlinesWideScaledLines(t *testing.T) {
	for _, test := range []struct{ width, scale int }{{4600, 1}, {2300, 2}, {2300, 3}} {
		t.Run(fmt.Sprintf("%d@%d", test.width, test.scale), func(t *testing.T) {
			d := NewDocument(NewText(strings.Repeat("word ", 400)))
			d.SetSelection(0, 0)
			tt := ui.NewTester(func(c *ui.Context) {
				EditorBase(c, d, EditorOptions{Font: ui.Font{Size: 20}, Underlines: func(text Text, paragraph int) []Underline {
					return []Underline{{Start: 0, End: text.Len(), Wavy: true, Color: ui.RGB(240, 0, 0)}}
				}}).Fill()
			}, test.width, 100)
			tt.SetScale(float32(test.scale))
			tt.Frame()
			points := underlinePixels(tt.Image())
			if len(points) == 0 {
				t.Fatal("wide line lost its wave")
			}
			firstY := points[0].Y
			bins := map[int]int{}
			for _, point := range points {
				if point.Y <= firstY+3*test.scale {
					bins[point.X/128]++
				}
			}
			// Every interior band of the first visual line has a visible wave,
			// including the boundaries where the renderer's paths are split.
			for bin := 1; bin < (test.width*test.scale-128)/128; bin++ {
				if bins[bin] < 3 {
					t.Errorf("wave missing in device-pixel band %d", bin)
				}
			}
		})
	}
}

func underlinePixels(img *image.RGBA) []image.Point {
	var points []image.Point
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R > 180 && c.G < 100 && c.B < 100 {
				points = append(points, image.Pt(x, y))
			}
		}
	}
	return points
}

func TestUnderlinesAreTemporary(t *testing.T) {
	for _, wavy := range []bool{false, true} {
		t.Run(map[bool]string{false: "straight", true: "wavy"}[wavy], func(t *testing.T) {
			d := NewDocument(ParseMarkdown("A **wrng** word and `code`."))
			before := d.Text()
			original := before.Markdown()
			d.SetSelection(2, 6)
			on := false
			opts := EditorOptions{Underlines: func(text Text, p int) []Underline {
				if !text.Equal(before) {
					t.Error("callback text differs from the displayed document")
				}
				if !on {
					return nil
				}
				return []Underline{{Start: 2, End: 6, Color: ui.RGB(240, 0, 0), Wavy: wavy}}
			}}
			tt := baseTester(d, opts)
			plain := bytes.Clone(tt.Image().Pix)
			on = true
			tt.Frame()
			if len(underlinePixels(tt.Image())) == 0 {
				t.Fatal("mark was not painted")
			}
			if !d.Text().Equal(before) || d.Text().Markdown() != original || d.CanUndo() {
				t.Fatal("painting changed text, serialized bytes or undo history")
			}
			if a, c := d.Selection(); a != 2 || c != 6 {
				t.Fatal("painting moved the selection")
			}
			on = false
			tt.Frame()
			if !bytes.Equal(plain, tt.Image().Pix) {
				t.Fatal("cleared marks left pixels or changed layout")
			}
		})
	}
}

func TestUnderlinesOnlyRequestVisibleParagraphs(t *testing.T) {
	const paragraph = "A paragraph with some words to check.\n"
	d := NewDocument(NewText(strings.Repeat(paragraph, 30000)))
	var s Scroll
	seen := map[int]bool{}
	opts := EditorOptions{Scroll: &s, Underlines: func(text Text, p int) []Underline {
		seen[p] = true
		if text.ParagraphStart(p) != p*len(paragraph) {
			t.Errorf("unexpected paragraph offset %d", text.ParagraphStart(p))
		}
		return nil
	}}
	tt := baseTester(d, opts)
	if len(seen) == 0 || len(seen) > 40 || seen[1000] {
		t.Fatalf("checked %d paragraphs before scrolling", len(seen))
	}
	seen = map[int]bool{}
	s.ScrollTo(20000)
	tt.Frame()
	if len(seen) == 0 || len(seen) > 40 || seen[0] {
		t.Fatalf("checked %d paragraphs after scrolling, first=%v", len(seen), seen[0])
	}
}

func TestUnderlinesUnicodeWrappingAndClipping(t *testing.T) {
	d := NewDocument(NewText("🙂猫 " + strings.Repeat("wrng ", 30) + "\nnext"))
	d.SetSelection(0, 0)
	var marks []Underline
	var scroll Scroll
	opts := EditorOptions{Scroll: &scroll, Font: ui.Font{Size: 20}, MaxWidth: 140, Underlines: func(text Text, p int) []Underline { return marks }}
	tt := baseTester(d, opts)
	plain := bytes.Clone(tt.Image().Pix)
	marks = []Underline{{Start: 3, End: 7, Wavy: true, Color: ui.RGB(240, 0, 0)}}
	tt.Frame()
	points := underlinePixels(tt.Image())
	if len(points) == 0 {
		t.Fatal("no mark after supplementary and CJK runes")
	}
	for _, p := range points {
		i, ok := scroll.IndexAt(float32(p.X)+0.5, float32(p.Y)-3)
		if !ok || i < 3 || i > 7 {
			t.Fatalf("mark at %v hits rune %d, outside wrng", p, i)
		}
	}
	wavy := bytes.Clone(tt.Image().Pix)
	marks[0].Wavy = false
	tt.Frame()
	if bytes.Equal(wavy, tt.Image().Pix) {
		t.Fatal("wavy and straight marks painted identically")
	}
	marks = []Underline{{Start: 3, End: 27, Wavy: true, Color: ui.RGB(240, 0, 0)}}
	tt.Frame()
	points = underlinePixels(tt.Image())
	if len(points) == 0 {
		t.Fatal("no Unicode-offset underline")
	}
	minY, maxY := points[0].Y, points[0].Y
	for _, p := range points {
		minY, maxY = min(minY, p.Y), max(maxY, p.Y)
	}
	if maxY-minY < 30 {
		t.Fatalf("mark did not wrap: y=%d..%d", minY, maxY)
	}
	// Empty, reversed and wholly outside ranges draw nothing, including at
	// the final empty paragraph. Broad valid ranges are clipped per paragraph.
	marks = []Underline{{Start: -10, End: -1}, {Start: 4, End: 4}, {Start: 9, End: 2}, {Start: d.Text().Len(), End: d.Text().Len() + 100}}
	tt.Frame()
	if !bytes.Equal(plain, tt.Image().Pix) {
		t.Fatal("invalid or out-of-text ranges painted")
	}
	marks = []Underline{{Start: -100, End: d.Text().Len() + 100, Wavy: true, Color: ui.RGB(240, 0, 0)}}
	tt.Frame()
	if len(underlinePixels(tt.Image())) == 0 {
		t.Fatal("clipped range did not paint")
	}
}

func TestUnderlinesCompositionAndHistory(t *testing.T) {
	d := NewDocument(NewText("wrng\nother"))
	seen := map[int]bool{}
	opts := EditorOptions{Underlines: func(text Text, p int) []Underline {
		seen[p] = true
		start := text.ParagraphStart(p)
		return []Underline{{Start: start, End: start + 4, Wavy: true, Color: ui.RGB(240, 0, 0)}}
	}}
	tt, _, _ := inputHookTester(t, d, &opts)
	d.SetSelection(2, 2)
	tt.Frame()
	seen = map[int]bool{}
	tt.Compose("字", 1)
	if seen[0] || !seen[1] {
		t.Fatalf("composition callbacks: %v", seen)
	}
	tt.Compose("", 0)
	tt.Type("x")
	seen = map[int]bool{}
	tt.Frame()
	if !seen[0] {
		t.Fatal("composition completion did not restore marks")
	}
	if d.Text().String() != "wrxng\nother" {
		t.Fatalf("typing: %q", d.Text().String())
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if d.Text().String() != "wrng\nother" || d.CanUndo() {
		t.Fatalf("painting split undo: %q", d.Text().String())
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	if d.Text().String() != "wrxng\nother" {
		t.Fatal("painting changed redo")
	}
}
