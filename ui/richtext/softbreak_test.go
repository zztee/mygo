package richtext

import "testing"

// A soft line break reads as a space that remembers it was a line's end,
// and is written back as one, wherever it is: in a list's item, a
// quotation, a mark.
func TestMarkdownKeepsSoftBreaks(t *testing.T) {
	for _, md := range []string{
		"one two\nthree\nfour",
		"- item\n  more\n- next",
		"1. one\n   more",
		"- [ ] task\n  more",
		"> quoted\n> more",
		"> - a\n>   b",
		"**bold\nbold** and *it\nalic*",
		"[a\nlink](https://x.org) after\nit",
		"a\\\nb\nc",
		"中文\nEnglish",
		"- a\n    - b\n      c",
	} {
		if got := ParseMarkdown(md).Markdown(); got != md {
			t.Errorf("%q came back\n%q", md, got)
		}
	}
	if got := ParseMarkdown("one\ntwo").String(); got != "one two" {
		t.Errorf("read as %q", got)
	}
	soft := Run{Text: " ", Attributes: Attributes{SoftBreak: true}}
	para := func(runs ...Run) Text { return TextOf(Paragraph{Runs: runs}) }
	for _, c := range []struct {
		text Text
		want string
	}{
		// What would start a block on the next line is escaped.
		{para(Run{Text: "a"}, soft, Run{Text: "# b"}), "a\n\\# b"},
		{para(Run{Text: "a"}, soft, Run{Text: "1. b"}), "a\n1\\. b"},
		{para(Run{Text: "a"}, soft, Run{Text: "    b"}), "a\n&#32;&#32;&#32;&#32;b"},
		// Spaces before it would make it a hard break.
		{para(Run{Text: "a  "}, soft, Run{Text: "b"}), "a\nb"},
		// None at the ends, and no blank line between two.
		{para(soft, Run{Text: "a"}, soft), "&#32;a"},
		{para(Run{Text: "a"}, soft, soft, Run{Text: "b"}), "a\nb"},
		{para(Run{Text: "a"}, soft, Run{Text: " "}, soft, Run{Text: "b"}), "a\nb"},
		// A break right after a hard one is a space.
		{para(Run{Text: "a "}, soft, Run{Text: "b"}), "a\\\n&#32;b"},
		// In a heading, a space.
		{TextOf(Paragraph{Style: ParagraphStyle{Heading: 2}, Runs: []Run{{Text: "a"}, soft, {Text: "b"}}}), "## a b"},
	} {
		if got := c.text.Markdown(); got != c.want {
			t.Errorf("%q: got %q, want %q", c.text.String(), got, c.want)
		}
	}
	// Typing after the break, or over it, makes no break.
	doc := NewDocument(ParseMarkdown("one\ntwo"))
	if a := typingAttributes(doc.Text(), 4, 4); a.SoftBreak {
		t.Fatal("typing after a soft break continues it")
	}
	if a := typingAttributes(doc.Text(), 3, 5); a.SoftBreak {
		t.Fatal("typing over a soft break continues it")
	}
	doc.SetSelection(1, 1)
	doc.InsertText("X")
	if got := doc.Text().Markdown(); got != "oXne\ntwo" {
		t.Fatalf("edited as %q", got)
	}
	doc.SetSelection(5, 5)
	doc.InsertText("Y")
	if got := doc.Text().Markdown(); got != "oXne\nYtwo" {
		t.Fatalf("typed after the break as %q", got)
	}
}
