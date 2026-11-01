package richtext

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// Wide soft breaks have a source position without adding a displayed rune.
func TestMarkdownKeepsWideSoftBreaks(t *testing.T) {
	for _, md := range []string{
		"第一行中文\n第二行中文\n第三行中文",
		"甲\n乙\n丙",
		"日本語の\nテキスト",
		"中文，\n继续",
		"第一行\n第二行 English\nlast line",
		"**粗体\n中文**和*斜体\n文字*",
		"**粗体**\n中文",
		"中文\n**粗体**",
		"[链接\n文字](https://example.org)",
		"中文\n`代码`",
		"`中文`\n`代码`",
		"`代码`\n中文",
		"中文\n<u>继续</u>",
		"> 引用\n> 继续",
		"- 列表\n  继续\n- 后续",
		"中文\\\n硬换行\n软换行",
	} {
		t.Run(md, func(t *testing.T) {
			text := ParseMarkdown(md)
			if got := text.Markdown(); got != md {
				t.Fatalf("round trip: got %q, want %q", got, md)
			}
			if strings.ContainsAny(text.String(), "\u2028\u2029\u200b") && !strings.Contains(md, "\\\n") {
				t.Fatal("soft break added a visible-text rune")
			}
			ps := make([]Paragraph, 0)
			for _, p := range text.Paragraphs() {
				p.Runs = append([]Run{{Text: "字"}}, p.Runs...)
				ps = append(ps, p)
			}
			edited := TextOf(ps...)
			back := ParseMarkdown(edited.Markdown())
			if back.String() != edited.String() {
				t.Fatalf("edited text changed: %q => %q", edited.String(), back.String())
			}
			if strings.Count(edited.Markdown(), "\n") != strings.Count(md, "\n") {
				t.Fatalf("editing lost a source line: %q", edited.Markdown())
			}
		})
	}
}

func TestWideSoftBreakEdits(t *testing.T) {
	const md = "甲乙丙\n丁戊己\n庚辛壬"
	for _, c := range []struct {
		name         string
		a, z         int
		insert, want string
	}{
		{"first-line", 1, 1, "字", "甲字乙丙\n丁戊己\n庚辛壬"},
		{"second-line", 4, 4, "字", "甲乙丙\n丁字戊己\n庚辛壬"},
		{"third-line", 8, 8, "字", "甲乙丙\n丁戊己\n庚辛字壬"},
		{"line-start", 3, 3, "字", "甲乙丙\n字丁戊己\n庚辛壬"},
		{"replace-line-start", 3, 4, "字文", "甲乙丙\n字文戊己\n庚辛壬"},
		{"delete-line-start", 3, 4, "", "甲乙丙\n戊己\n庚辛壬"},
		{"delete-line", 3, 6, "", "甲乙丙\n庚辛壬"},
		{"delete-across-break", 2, 4, "", "甲乙戊己\n庚辛壬"},
		{"latin-before-break", 3, 3, "word", "甲乙丙word丁戊己\n庚辛壬"},
		{"latin-line-end", 2, 3, "word", "甲乙word丁戊己\n庚辛壬"},
		{"new-paragraph-at-break", 3, 3, "\n", "甲乙丙\n\n丁戊己\n庚辛壬"},
	} {
		t.Run(c.name, func(t *testing.T) {
			doc := NewDocument(ParseMarkdown(md))
			doc.SetSelection(c.a, c.z)
			doc.InsertText(c.insert)
			if got := doc.Text().Markdown(); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
			doc.Undo()
			if got := doc.Text().Markdown(); got != md {
				t.Fatalf("undo lost source breaks: %q", got)
			}
			doc.Redo()
			if got := doc.Text().Markdown(); got != c.want {
				t.Fatalf("redo lost source breaks: %q", got)
			}
		})
	}
}

func TestWideSoftBreakEditor(t *testing.T) {
	doc := NewDocument(ParseMarkdown("甲乙丙\n丁戊己"))
	tt, _ := richTester(t, doc)
	doc.SetSelection(3, 3)
	tt.Frame()
	tt.Type("字文")
	if got := doc.Text().Markdown(); got != "甲乙丙\n字文丁戊己" {
		t.Fatalf("typed as %q", got)
	}
	doc.SetSelection(0, doc.Text().Len())
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyC)
	if got := tt.Clipboard(); got != "甲乙丙字文丁戊己" {
		t.Fatalf("plain clipboard changed: %q", got)
	}
	data, err := tt.ClipboardData().Read(Format)
	if err != nil {
		t.Fatal(err)
	}
	copied, ok := decodeClip(data)
	if !ok || copied.Markdown() != doc.Text().Markdown() {
		t.Fatal("rich clipboard lost breaks")
	}
}

func TestWideSoftBreakFormatting(t *testing.T) {
	for _, c := range []struct {
		name   string
		format func(*Attributes)
		want   string
	}{
		{"bold", func(a *Attributes) { a.Weight = 700 }, "**甲乙丙\n丁戊己**"},
		{"code", func(a *Attributes) { a.Code = true }, "`甲乙丙`\n`丁戊己`"},
	} {
		t.Run(c.name, func(t *testing.T) {
			doc := NewDocument(ParseMarkdown("甲乙丙\n丁戊己"))
			doc.SetSelection(0, doc.Text().Len())
			doc.Format(c.format)
			got := doc.Text().Markdown()
			if got != c.want || ParseMarkdown(got).String() != doc.Text().String() {
				t.Fatalf("formatted as %q, want %q", got, c.want)
			}
		})
	}
	ps := []Paragraph{}
	for _, p := range ParseMarkdown("甲乙丙\n丁戊己").Paragraphs() {
		p.Style.Heading = 2
		ps = append(ps, p)
	}
	if got := TextOf(ps...).Markdown(); got != "## 甲乙丙丁戊己" {
		t.Fatalf("heading acquired a space: %q", got)
	}
}

func TestWideSoftBreakOffsetsAndClipboard(t *testing.T) {
	for _, md := range []string{"甲\n乙\n丙", "甲乙\n丙", "甲\n乙丙"} {
		text := ParseMarkdown(md)
		if text.String() != "甲乙丙" || text.Len() != 3 || text.PlainText() != "甲乙丙" {
			t.Fatalf("wide break changed characters: %q", text.String())
		}
		clip := clipOf(text)
		clip.Copy = ""
		data, err := json.Marshal(clip)
		if err != nil {
			t.Fatal(err)
		}
		back, ok := decodeClip(data)
		if !ok || back.Markdown() != md || back.String() != text.String() {
			t.Fatalf("cross-process clipboard lost source boundaries: %q", back.Markdown())
		}
		if got := text.Slice(1, 3).Markdown(); got != strings.TrimPrefix(strings.TrimPrefix(md, "甲"), "\n") {
			t.Fatalf("sliced as %q", got)
		}
		if got := text.Markdown(); got != md {
			t.Fatalf("slice mutated original: %q", got)
		}
	}
}

func TestWideSoftBreakHTMLStyles(t *testing.T) {
	for _, c := range []struct {
		name   string
		z      int
		format func(*Attributes)
	}{
		{"underline", 2, func(a *Attributes) { a.Underline = true }},
		{"bold-fallback", 3, func(a *Attributes) { a.Weight = 700 }},
		{"italic-fallback", 3, func(a *Attributes) { a.Italic = true }},
	} {
		t.Run(c.name, func(t *testing.T) {
			text := ParseMarkdown("甲\n乙。丙").Format(1, c.z, c.format)
			md := text.Markdown()
			back := ParseMarkdown(md)
			if back.String() != text.String() || markKey(collectRuns(back)) != markKey(collectRuns(text)) {
				t.Fatalf("HTML style changed content or marks: %q => %q", md, back.String())
			}
			if !strings.Contains(md, "\n") {
				t.Fatalf("HTML style lost the boundary: %q", md)
			}
		})
	}
}

func collectRuns(text Text) []Run {
	var runs []Run
	for _, p := range text.Paragraphs() {
		runs = append(runs, p.Runs...)
	}
	return runs
}

func TestWideSoftBreakComposeSelection(t *testing.T) {
	for _, end := range []int{2, 3} {
		for _, commit := range []string{"direct", "empty-mark"} {
			t.Run(commit+string(rune('0'+end)), func(t *testing.T) {
				doc := NewDocument(ParseMarkdown("甲\n乙丙"))
				tt, _ := richTester(t, doc)
				doc.SetSelection(1, end)
				tt.Frame()
				tt.Compose("bing", 4)
				if commit == "empty-mark" {
					tt.Compose("", 0)
				}
				tt.Type("丁")
				want := "甲\n丁"
				if end == 2 {
					want += "丙"
				}
				if got := doc.Text().Markdown(); got != want {
					t.Fatalf("IME committed as %q, want %q", got, want)
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				if commit == "empty-mark" {
					// Ending an empty composition cancels it; later typing
					// has its own step, after the selection's deletion.
					deleted := "甲"
					if end == 2 {
						deleted += "\n丙"
					}
					if got := doc.Text().Markdown(); got != deleted {
						t.Fatalf("cancelled IME undo: %q", got)
					}
					tt.Key(ui.Cmd, ui.KeyZ)
				}
				if got := doc.Text().Markdown(); got != "甲\n乙丙" {
					t.Fatalf("IME undo: %q", got)
				}
			})
		}
	}
}

func TestWideSoftBreakSelectionPaste(t *testing.T) {
	doc := NewDocument(ParseMarkdown("甲\n乙\n丙丁"))
	tt, _ := richTester(t, doc)
	doc.SetSelection(1, 3)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyC)
	original := tt.ClipboardData()
	encoded, err := original.Read(Format)
	if err != nil {
		t.Fatal(err)
	}
	var clip clipText
	if err := json.Unmarshal(encoded, &clip); err != nil {
		t.Fatal(err)
	}
	clip.Copy = ""
	encoded, err = json.Marshal(clip)
	if err != nil {
		t.Fatal(err)
	}
	for _, cross := range []bool{false, true} {
		target := NewDocument(NewText("戊"))
		paste, _ := richTester(t, target)
		paste.SetClipboardData(original)
		if cross {
			paste.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(Format, encoded))))
		}
		paste.Key(ui.Cmd, ui.KeyV)
		if got := target.Text().Markdown(); got != "戊乙\n丙" {
			t.Fatalf("cross=%v pasted as %q", cross, got)
		}
	}
	tt.Key(ui.Cmd, ui.KeyX)
	if got := doc.Text().Markdown(); got != "甲\n丁" {
		t.Fatalf("cut lost boundary at source: %q", got)
	}
	doc.SetSelection(doc.Text().Len(), doc.Text().Len())
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyV)
	if got := doc.Text().Markdown(); got != "甲\n丁乙\n丙" {
		t.Fatalf("cut-paste duplicated boundary: %q", got)
	}
}
