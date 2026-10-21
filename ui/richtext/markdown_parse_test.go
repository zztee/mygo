package richtext

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

// dump writes the paragraphs of t, their styles and runs, to compare.
func dump(t Text) string {
	var b strings.Builder
	for _, p := range t.Paragraphs() {
		fmt.Fprintf(&b, "%+v:", p.Style)
		for _, r := range p.Runs {
			a := r.Attributes
			at := a.Attachment
			a.Attachment = nil
			fmt.Fprintf(&b, " %q%+v", r.Text, a)
			if at != nil {
				fmt.Fprintf(&b, "[%q %q]", at.Name, at.URL)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestParseMarkdown(t *testing.T) {
	want := sample()
	// Markdown has no alignment.
	var paras []Paragraph
	for _, p := range want.Paragraphs() {
		p.Style.Align = ui.Start
		paras = append(paras, p)
	}
	want = TextOf(paras...)
	if got := ParseMarkdown(sample().Markdown()); dump(got) != dump(want) {
		t.Errorf("got\n%s\nwant\n%s", dump(got), dump(want))
	}

	para := func(st ParagraphStyle, runs ...Run) Paragraph { return Paragraph{Style: st, Runs: runs} }
	bold := Attributes{Weight: 700}
	for _, c := range []struct {
		src  string
		want Text
	}{
		{"- item\n\n  more\n\n      code\n- next", TextOf(
			para(ParagraphStyle{List: Bullets, Loose: true}, Run{Text: "item"}),
			para(ParagraphStyle{Indent: 1}, Run{Text: "more"}),
			para(ParagraphStyle{Indent: 1, CodeBlock: true}, Run{Text: "code"}),
			para(ParagraphStyle{List: Bullets, Loose: true}, Run{Text: "next"}),
		)},
		{"> - [x] done\n>   - nested", TextOf(
			para(ParagraphStyle{List: Tasks, Checked: true, Quote: true}, Run{Text: "done"}),
			para(ParagraphStyle{List: Bullets, Indent: 1, Quote: true}, Run{Text: "nested"}),
		)},
		{"a  \nb", TextOf(para(ParagraphStyle{}, Run{Text: "a\u2028b"}))},
		{"1. a\\\n   b", TextOf(para(ParagraphStyle{List: Numbers}, Run{Text: "a\u2028b"}))},
		{"a<br>b", TextOf(para(ParagraphStyle{}, Run{Text: "a\u2028b"}))},
		{"| a | b |\n|---|---|\n| 1 | **2** |", TextOf(
			para(ParagraphStyle{}, Run{Text: "a\tb", Attributes: bold}),
			para(ParagraphStyle{}, Run{Text: "1\t"}, Run{Text: "2", Attributes: bold}),
		)},
		{"x <u>under *it*</u> <b>b</b>", TextOf(para(ParagraphStyle{}, Run{Text: "x "}, Run{Text: "under ", Attributes: Attributes{Underline: true}},
			Run{Text: "it", Attributes: Attributes{Underline: true, Italic: true}}, Run{Text: " "}, Run{Text: "b", Attributes: bold}))},
		{"![a *cat*](cat.png \"t\") [x](javascript:alert(1))", TextOf(para(ParagraphStyle{},
			Run{Text: string(ObjectReplacement), Attributes: Attributes{Attachment: &Attachment{Name: "a cat", URL: "cat.png"}}},
			Run{Text: " x"}))},
		{"see $$x^2$$, \\[a\\] and[^1]\n\n[^1]: The note.", TextOf(
			para(ParagraphStyle{}, Run{Text: "see $$x^2$$, [a] and[1]"}),
			para(ParagraphStyle{}, Run{Text: "[1] The note."}),
		)},
		{"", Text{}},
	} {
		if got := ParseMarkdown(c.src); dump(got) != dump(c.want) {
			t.Errorf("%q:\ngot\n%s\nwant\n%s", c.src, dump(got), dump(c.want))
		}
	}
}

// TestMarkdownRoundTrip writes random texts as Markdown and reads them
// back: what Markdown keeps comes back the same.
func TestMarkdownRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	words := []string{"foo", "bar", "a", "x1", ".", ",", "!", "?", "(", ")", "-", "#", "*", "_", "`", "~", "[", "]", "<", ">", "&", "\\", "1.", "é", "\"", "'", ":", "/"}
	pick := func() Attributes {
		var a Attributes
		if rng.IntN(3) == 0 {
			a.Weight = 700
		}
		if rng.IntN(3) == 0 {
			a.Italic = true
		}
		if rng.IntN(5) == 0 {
			a.Strikethrough = true
		}
		if rng.IntN(6) == 0 {
			a.Underline = true
		}
		if rng.IntN(6) == 0 {
			a.Link = "https://e.org/" + fmt.Sprint(rng.IntN(3))
		}
		if rng.IntN(7) == 0 {
			a = Attributes{Code: true}
		}
		return a
	}
	failures := 0
	for range 3000 {
		var runs []Run
		for range 1 + rng.IntN(5) {
			var s strings.Builder
			for k := range 1 + rng.IntN(3) {
				if k > 0 && rng.IntN(2) == 0 {
					s.WriteByte(' ')
				}
				s.WriteString(words[rng.IntN(len(words))])
			}
			runs = append(runs, Run{Text: s.String(), Attributes: pick()})
		}
		// Only what shows the same: spaces at the ends of a paragraph are
		// not shown, and a code span's are its own.
		want := TextOf(Paragraph{Runs: runs})
		want = TextOf(normalize(want)...)
		md := want.Markdown()
		if got := ParseMarkdown(md); dump(got) != dump(want) {
			failures++
			if failures <= 10 {
				t.Errorf("%q as Markdown %q reads\n%s want\n%s", want.String(), md, dump(got), dump(want))
			}
		}
	}
	if failures > 0 {
		t.Errorf("%d of 3000 failed", failures)
	}
}

// normalize returns the paragraphs of t as Markdown can keep them: no
// spaces at their ends, and none at the ends of code, which a code span
// takes as its padding.
func normalize(t Text) []Paragraph {
	var out []Paragraph
	for _, p := range t.Paragraphs() {
		var runs []Run
		for i, r := range p.Runs {
			if i == 0 {
				r.Text = strings.TrimLeft(r.Text, " ")
			}
			if i == len(p.Runs)-1 {
				r.Text = strings.TrimRight(r.Text, " ")
			}
			if r.Attributes.Code {
				r.Text = strings.Trim(r.Text, " ")
			}
			if r.Text == "" {
				continue
			}
			if n := len(runs); n > 0 && runs[n-1].Attributes == r.Attributes {
				runs[n-1].Text += r.Text
				continue
			}
			runs = append(runs, r)
		}
		p.Runs = runs
		out = append(out, p)
	}
	return out
}

// TestPasteFromMarkdownView copies a Markdown view's selection and pastes
// it into the editor with its formatting, through HTML, and pastes the
// Markdown that Markdown apps copy.
func TestPasteFromMarkdownView(t *testing.T) {
	src := "## Plan\n\nSome **bold** and `code`.\n\n- [x] done\n"
	view := ui.NewTester(func(c *ui.Context) { markdown.View(c, src, nil) }, 500, 300)
	view.Click("Plan")
	view.Key(ui.Cmd, ui.KeyA)
	view.Key(ui.Cmd, ui.KeyC)
	data := view.ClipboardData()
	for _, f := range []transfer.Format{transfer.Text, transfer.HTML, transfer.Markdown} {
		if _, err := data.Read(f); err != nil {
			t.Fatalf("no %s copied", f)
		}
	}
	want := TextOf(
		Paragraph{Style: ParagraphStyle{Heading: 2}, Runs: []Run{{Text: "Plan"}}},
		Paragraph{Runs: []Run{{Text: "Some "}, {Text: "bold", Attributes: Attributes{Weight: 700}}, {Text: " and "}, {Text: "code", Attributes: Attributes{Code: true}}, {Text: "."}}},
		Paragraph{Style: ParagraphStyle{List: Tasks, Checked: true}, Runs: []Run{{Text: "done"}}},
	)
	for _, only := range []transfer.Format{transfer.HTML, transfer.Markdown} {
		b, _ := data.Read(only)
		doc := NewDocument(Text{})
		tt, _ := richTester(t, doc)
		tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(only, b), transfer.Bytes(transfer.Text, []byte("plain")))))
		tt.Key(ui.Cmd, ui.KeyV)
		if dump(doc.Text()) != dump(want) {
			t.Errorf("pasted from %s %q:\n%s\nwant\n%s", only, b, dump(doc.Text()), dump(want))
		}
	}
}

// What a list's item, a quotation and a paragraph hold comes back as it
// was: hard line breaks, paragraphs and code under an item, numbering on
// past them, and a quotation's paragraphs in one quotation.
func TestMarkdownKeepsBlockShapes(t *testing.T) {
	for _, md := range []string{
		"a\\\nb",
		"- one\\\n  two\n- three",
		"1. one\\\n   two",
		"- [ ] task\\\n  more",
		"> quoted\\\n> on",
		"1. first\n\n    more of it\n\n2. second",
		"- item\n\n    ```go\n    x := 1\n\n    y := 2\n    ```\n\n- next",
		"> one\n>\n> two",
		"- item\n\n    > quoted\n    >\n    > more\n\n- next",
		"> - item\n>\n>     more",
		"- a\n    - b\n\n        under b\n\n    - c",
		"# Title<br>more",
		"line\\\n\\# not a heading",
	} {
		if got := ParseMarkdown(md).Markdown(); got != md {
			t.Errorf("%q came back\n%q", md, got)
		}
	}
	// A break at the end of a paragraph is HTML's, which Markdown keeps.
	if got := TextOf(Paragraph{Runs: []Run{{Text: "end "}}}).Markdown(); got != "end<br>" {
		t.Errorf("trailing break: %q", got)
	}
}

// A list comes back in the form it was written: its markers, the blank
// lines between its items, numbered tasks, and a count from 0.
func TestMarkdownKeepsListForms(t *testing.T) {
	for _, md := range []string{
		"1. [ ] first\n2. [x] second",
		"1) [ ] first\n2) [ ] second",
		"0. zero\n1. one\n2. two",
		"- one\n\n- two\n\n- three",
		"1. one\n\n2. two",
		"- dash\n- dash\n\n* star\n* star",
		"1. one\n2. two\n\n1) uno\n2) dos",
		"1) one\n2) two",
		"+ plus\n    * inner",
		"* [x] done",
		"> - a\n>\n> * b",
		"> 1. one\n>\n> 2. two",
		"- a\n1. one",
	} {
		if got := ParseMarkdown(md).Markdown(); got != md {
			t.Errorf("%q came back\n%q", md, got)
		}
	}
}

// Links and images keep their titles, and an image keeps the link
// around it.
func TestMarkdownKeepsTitlesAndLinkedImages(t *testing.T) {
	for _, md := range []string{
		`See [Example](https://example.com "Example Title").`,
		`![a picture](assets/pic.png "Picture title")`,
		`[![a picture](assets/pic.png)](https://example.com)`,
		`[![p](i.png "t")](https://x.org "u")`,
		`[a](https://x.org "say \"hi\"")`,
	} {
		if got := ParseMarkdown(md).Markdown(); got != md {
			t.Errorf("%q came back\n%q", md, got)
		}
	}
	// A title goes with its link: typing after it, or another address,
	// leaves it behind.
	d := NewDocument(ParseMarkdown(`[a](https://x.org "t")`))
	d.SetSelection(0, 1)
	d.SetLink("https://y.org")
	if got := d.Text().Markdown(); got != "[a](https://y.org)" {
		t.Errorf("relinked %q", got)
	}
}

// A quotation inside a quotation stays inside it.
func TestMarkdownKeepsNestedQuotes(t *testing.T) {
	for _, md := range []string{
		"> Outer\n>\n> > Inner",
		"> a\n>\n> > b\n>\n> c",
		"> > only deep",
		"> one\n>\n> > two\n> >\n> > > three",
	} {
		if got := ParseMarkdown(md).Markdown(); got != md {
			t.Errorf("%q came back\n%q", md, got)
		}
	}
	// Taking the style off a nested quotation's paragraph takes one level.
	s := ParagraphStyle{Quote: true, QuoteDepth: 1}
	unstyle(&s)
	if !s.Quote || s.QuoteDepth != 0 {
		t.Errorf("unstyled %+v", s)
	}
}
