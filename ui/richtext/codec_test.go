package richtext

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// sample is a text of every kind of paragraph and run.
func sample() Text {
	bold := Attributes{Weight: 700}
	return TextOf(
		Paragraph{Style: ParagraphStyle{Heading: 1}, Runs: []Run{{Text: "Trip"}}},
		Paragraph{Runs: []Run{{Text: "Land at "}, {Text: "10:40", Attributes: bold}, {Text: " near the "},
			{Text: "castle", Attributes: Attributes{Link: "https://x.org/a"}}, {Text: ", "}, {Text: "slowly", Attributes: Attributes{Italic: true}}, {Text: "."}}},
		Paragraph{Style: ParagraphStyle{List: Tasks, Checked: true}, Runs: []Run{{Text: "Passport"}}},
		Paragraph{Style: ParagraphStyle{List: Tasks}, Runs: []Run{{Text: "Adapter"}}},
		Paragraph{Style: ParagraphStyle{List: Numbers}, Runs: []Run{{Text: "One"}}},
		Paragraph{Style: ParagraphStyle{List: Numbers, Indent: 1}, Runs: []Run{{Text: "Inner"}}},
		Paragraph{Style: ParagraphStyle{List: Numbers}, Runs: []Run{{Text: "Two"}}},
		Paragraph{Style: ParagraphStyle{Quote: true}, Runs: []Run{{Text: "A quote"}}},
		Paragraph{Style: ParagraphStyle{CodeBlock: true}, Runs: []Run{{Text: "a := 1"}}},
		Paragraph{Style: ParagraphStyle{CodeBlock: true}, Runs: []Run{{Text: "b := 2"}}},
		Paragraph{Style: ParagraphStyle{Align: ui.Center}, Runs: []Run{{Text: "mid", Attributes: Attributes{Underline: true, Strikethrough: true}}, {Text: " and "}, {Text: "x()", Attributes: Attributes{Code: true}}}},
	)
}

func TestMarkdown(t *testing.T) {
	want := strings.Join([]string{
		"# Trip",
		"",
		"Land at **10:40** near the [castle](https://x.org/a), *slowly*.",
		"",
		"- [x] Passport",
		"- [ ] Adapter",
		"1. One",
		"    1. Inner",
		"2. Two",
		"",
		"> A quote",
		"",
		"```",
		"a := 1",
		"b := 2",
		"```",
		"",
		"~~<u>mid</u>~~ and `x()`",
	}, "\n")
	if got := sample().Markdown(); got != want {
		t.Fatalf("Markdown:\n%s\nwant:\n%s", got, want)
	}
	// What Markdown would read as a block is escaped in a body paragraph.
	if got := NewText("# not a heading").Markdown(); got != `\# not a heading` {
		t.Fatalf("%q", got)
	}
}

func TestHTMLRoundTrip(t *testing.T) {
	in := sample()
	html := in.HTML()
	out := ParseHTML(html)
	if out.String() != in.String() {
		t.Fatalf("text %q, want %q\nHTML %s", out.String(), in.String(), html)
	}
	if got, want := runsOf(out), runsOf(in); got != want {
		t.Fatalf("runs %q, want %q", got, want)
	}
	for start, p := range in.Paragraphs() {
		if got := out.ParagraphStyleAt(start); got != p.Style {
			t.Fatalf("paragraph at %d: %+v, want %+v", start, got, p.Style)
		}
	}
}

func TestParseForeignHTML(t *testing.T) {
	// As Google Docs copies: a bold wrapper made normal, spans styled.
	gdocs := `<meta charset="utf-8"><b style="font-weight:normal;" id="docs-internal-guid-1"><p dir="ltr"><span style="font-size:11pt;font-family:Arial;font-weight:700;">Bold</span><span style="font-size:11pt;"> and </span><span style="font-style:italic;">italic</span></p><ul><li><p><span>item</span></p></li></ul></b>`
	got := ParseHTML(gdocs)
	if s := got.String(); s != "Bold and italic\nitem" {
		t.Fatalf("text %q", s)
	}
	if r := runsOf(got.Slice(0, 15)); r != "*Bold*| and |_italic_" {
		t.Fatalf("runs %q", r)
	}
	if got.ParagraphStyleAt(17).List != Bullets {
		t.Fatalf("not a list: %+v", got.ParagraphStyleAt(17))
	}
	// Whitespace collapses; scripts and styles are left out.
	got = ParseHTML("<style>p{}</style><p>  a \n  b  </p><script>x()</script><p>c<br>d</p>")
	if s := got.String(); s != "a b\nc\nd" {
		t.Fatalf("text %q", s)
	}
}

func TestCopyPaste(t *testing.T) {
	doc := NewDocument(sample())
	tt, _ := richTester(t, doc)
	doc.SetSelection(5, 18) // "Land at 10:40"
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyC)
	d := tt.ClipboardData()
	for _, f := range []transfer.Format{Format, transfer.HTML, transfer.Text} {
		if _, err := d.Read(f); err != nil {
			t.Fatalf("no %s copied", f)
		}
	}
	if text, _ := d.Read(transfer.Text); string(text) != "Land at 10:40" {
		t.Fatalf("plain text %q", text)
	}
	doc.SetSelection(0, 0)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyV)
	if got := doc.Text().Slice(0, 13); runsOf(got) != "Land at |*10:40*" {
		t.Fatalf("pasted %q", runsOf(got))
	}
	// One step of undo takes the paste back.
	tt.Key(ui.Cmd, ui.KeyZ)
	if !doc.Text().Equal(sample()) {
		t.Fatalf("undo made %q", doc.Text().String())
	}
	// HTML from another app pastes its formats; Paste and Match Style
	// pastes its text in the typing attributes.
	tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.HTML, []byte("<b>hi</b>")), transfer.Bytes(transfer.Text, []byte("hi")))))
	doc.SetSelection(0, 0)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyV)
	if got := runsOf(doc.Text().Slice(0, 2)); got != "*hi*" {
		t.Fatalf("pasted HTML %q", got)
	}
	tt.Command("pasteAndMatchStyle")
	if got := runsOf(doc.Text().Slice(0, 4)); got != "*hihi*" {
		t.Fatalf("pasted matching the style %q", got)
	}
}

// OnPaste changes what a paste inserts, as an app leaves out what its
// files cannot keep: here the alignment HTML brings.
func TestOnPaste(t *testing.T) {
	doc := NewDocument(NewText("x"))
	var seen Text
	tt := ui.NewTester(func(c *ui.Context) {
		EditorBase(c, doc, EditorOptions{OnPaste: func(t Text) Text {
			seen = t
			var ps []Paragraph
			for _, p := range t.Paragraphs() {
				p.Style.Align = ui.Start
				ps = append(ps, p)
			}
			return TextOf(ps...)
		}}).Fill()
	}, 500, 400)
	tt.Press(250, 380)
	tt.Release(250, 380)
	tt.SetClipboardData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.HTML, []byte(`<p style="text-align:center">mid</p><p align="right"><b>end</b></p>`)))))
	doc.SetSelection(0, 0)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyV)
	if seen.ParagraphStyleAt(0).Align != ui.Center {
		t.Fatalf("OnPaste given %+v", seen.ParagraphStyleAt(0))
	}
	got := doc.Text()
	if got.String() != "mid\nendx" {
		t.Fatalf("pasted %q", got.String())
	}
	for start, p := range got.Paragraphs() {
		if p.Style.Align != ui.Start {
			t.Fatalf("paragraph at %d aligned %v", start, p.Style.Align)
		}
	}
	if r := runsOf(got.Slice(4, 7)); r != "*end*" {
		t.Fatalf("runs %q", r)
	}
}

func TestJSON(t *testing.T) {
	in := sample()
	b, err := in.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var out Text
	if err := out.UnmarshalJSON(b); err != nil {
		t.Fatal(err)
	}
	if !out.Equal(in) {
		t.Fatalf("%q, want %q", runsOf(out), runsOf(in))
	}
}

func TestParseRTF(t *testing.T) {
	// As TextEdit copies: a font table, bold, italic, a link, a list.
	rtf := `{\rtf1\ansi\ansicpg1252\cocoartf2822
{\fonttbl\f0\fswiss\fcharset0 Helvetica;}
{\colortbl;\red255\green255\blue255;}
\pard\tx560\pardirnatural\partightenfactor0
\f0\fs24 \cf0 Plain \b bold\b0  and \i italic\i0 , caf\'e9 \u8364 ? {\field{\*\fldinst{HYPERLINK "https://go.dev"}}{\fldrslt site}} end.\
\pard\qc centered\
\pard\tx220\li720\fi-720\pardirnatural\partightenfactor0
\ls1\ilvl0{\listtext	\uc0舦 	}one\
{\listtext	\uc0舦 	}two\
}`
	got := ParseRTF([]byte(rtf))
	if s := got.String(); s != "Plain bold and italic, café € site end.\ncentered\none\ntwo" {
		t.Fatalf("text %q", s)
	}
	first, _ := got.ParagraphRange(0)
	if r := runsOf(got.Slice(first, 39)); r != "Plain |*bold*| and |_italic_|, café € |[site]| end." {
		t.Fatalf("runs %q", r)
	}
	if st := got.ParagraphStyleAt(45); st.Align != ui.Center {
		t.Fatalf("centered: %+v", st)
	}
	if st := got.ParagraphStyleAt(got.Len()); st.List != Bullets {
		t.Fatalf("list: %+v", st)
	}
}

// TestMarkdownEscapes checks that what Markdown would read as more than
// text stays text: block starts in items and headings, a heading's
// closing #s, indentation, entities, images and backticks in code.
func TestMarkdownEscapes(t *testing.T) {
	para := func(st ParagraphStyle, runs ...Run) Paragraph { return Paragraph{Style: st, Runs: runs} }
	for _, c := range []struct {
		text Text
		want string
	}{
		{TextOf(para(ParagraphStyle{List: Bullets}, Run{Text: "# not a heading"})), `- \# not a heading`},
		{TextOf(para(ParagraphStyle{List: Numbers}, Run{Text: "> x"})), `1. \> x`},
		{TextOf(para(ParagraphStyle{Heading: 2}, Run{Text: "Issue #"})), `## Issue \#`},
		{NewText("---"), `\---`},
		{NewText("-"), `\-`},
		{NewText("    indented"), `&#32;&#32;&#32;&#32;indented`},
		{NewText("Tom &amp; Jerry & co"), `Tom \&amp; Jerry & co`},
		{NewText(`C:\Users\ike 桌面\笔记`), `C:\Users\ike 桌面\笔记`},
		{NewText(`a\*b\ end\`), `a\\\*b\ end\\`},
		{TextOf(para(ParagraphStyle{}, Run{Text: "x!"}, Run{Text: "y", Attributes: Attributes{Link: "http://e"}})), `x\![y](http://e)`},
		// A ! escaped only where a [ right after it would make an image.
		{NewText("Hello!"), `Hello!`},
		{NewText("Hi! there! Next!"), "Hi! there!\\\nNext!"},
		{TextOf(para(ParagraphStyle{}, Run{Text: "x! "}, Run{Text: "y", Attributes: Attributes{Link: "http://e"}})), `x! [y](http://e)`},
		{TextOf(para(ParagraphStyle{}, Run{Text: "x!", Attributes: Attributes{Weight: 700}}, Run{Text: "y", Attributes: Attributes{Link: "http://e"}})), `**x!**[y](http://e)`},
		{TextOf(para(ParagraphStyle{}, Run{Text: "x!"}, Run{Text: "y", Attributes: Attributes{Link: "http://e", Weight: 700}})), `x\![**y**](http://e)`},
		{TextOf(para(ParagraphStyle{}, Run{Text: "x!"}, Run{Text: string(ObjectReplacement), Attributes: Attributes{Attachment: &Attachment{Name: "a", URL: "i.png"}}})), `x!![a](i.png)`},
		{TextOf(para(ParagraphStyle{}, Run{Text: "x!"}, Run{Text: string(ObjectReplacement), Attributes: Attributes{Link: "http://e", Attachment: &Attachment{Name: "a", URL: "i.png"}}})), `x\![![a](i.png)](http://e)`},
		{TextOf(para(ParagraphStyle{}, Run{Text: "x!"}, Run{Text: "[y]"})), `x!\[y\]`},
		{TextOf(para(ParagraphStyle{}, Run{Text: "a``b", Attributes: Attributes{Code: true}})), "```a``b```"},
		{TextOf(para(ParagraphStyle{}, Run{Text: "`x", Attributes: Attributes{Code: true}})), "`` `x ``"},
		// Delimiters that cannot open or close where they are.
		{TextOf(para(ParagraphStyle{}, Run{Text: "foo.", Attributes: Attributes{Weight: 700}}, Run{Text: "bar"})), "<strong>foo.</strong>bar"},
		{TextOf(para(ParagraphStyle{}, Run{Text: "x"}, Run{Text: "(y)", Attributes: Attributes{Italic: true}})), "x<em>(y)</em>"},
		{TextOf(para(ParagraphStyle{}, Run{Text: "foo.", Attributes: Attributes{Weight: 700}}, Run{Text: " bar"})), "**foo.** bar"},
	} {
		if got := c.text.Markdown(); got != c.want {
			t.Errorf("%q: got %q, want %q", c.text.String(), got, c.want)
		}
	}
}

// TestCodecsSanitize checks what pasted documents may not bring: styles
// out of range, links that run scripts, sizes that aren't, pictures too
// large to decode, and another app's copy taken for this one's.
func TestCodecsSanitize(t *testing.T) {
	at, ok := decodeClip([]byte(`{"paragraphs":[{"style":{"list":1,"indent":-1,"heading":99},"runs":[{"text":"x","attributes":{"link":" javascript:alert(1)"}}]}]}`))
	if !ok {
		t.Fatal("not decoded")
	}
	if st := at.ParagraphStyleAt(0); st.Indent != 0 || st.Heading != 6 {
		t.Errorf("style %+v", st)
	}
	_ = at.HTML()
	_ = at.Markdown()
	for _, src := range []string{`<a href=" javascript:alert(1)">x</a>`, "<a href=\"java\tscript:alert(1)\">x</a>", `<a href="VBScript:x">x</a>`} {
		if l := ParseHTML(src).AttributesAt(0).Link; l != "" {
			t.Errorf("%s: link %q", src, l)
		}
	}
	if l := ParseRTF([]byte(`{\rtf1{\field{\*\fldinst HYPERLINK "javascript:alert(1)"}{\fldrslt x}}}`)).AttributesAt(0).Link; l != "" {
		t.Errorf("RTF link %q", l)
	}
	if h := TextOf(Paragraph{Runs: []Run{{Text: "x", Attributes: Attributes{Link: "javascript:alert(1)"}}}}).HTML(); strings.Contains(h, "javascript") {
		t.Errorf("HTML %s", h)
	}
	img := ParseHTML(`<img src="x.png" width="NaN" height="-Inf">`)
	if a := img.AttributesAt(0).Attachment; a == nil || a.Width != 0 || a.Height != 0 {
		t.Errorf("attachment %+v", a)
	} else if _, err := img.MarshalJSON(); err != nil {
		t.Error(err)
	}
	// Another app's copy with the name of this one's is its own text.
	encodeClip(NewText("my old copy"))
	if got, _ := decodeClip([]byte(`{"copy":"1","paragraphs":[{"runs":[{"text":"theirs"}]}]}`)); got.String() != "theirs" {
		t.Errorf("pasted %q", got.String())
	}
}

// TestImageDataURLs checks that a picture may be held in a data: URL of an
// image, an SVG too, as Markdown, HTML and this editor's copies give it,
// and is written out so; a link to a data: URL, a data: URL of another
// type and a script are not taken.
func TestImageDataURLs(t *testing.T) {
	png := "data:image/png;base64,iVBORw0KGgo="
	svg := "data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%2F%3E"
	for _, c := range []struct{ src, want string }{
		{png, png},
		{svg, svg},
		{"data:image/svg+xml;charset=utf-8;base64,PHN2Zy8+", "data:image/svg+xml;charset=utf-8;base64,PHN2Zy8+"},
		{"DATA:IMAGE/GIF;BASE64,R0lGODlh", "DATA:IMAGE/GIF;BASE64,R0lGODlh"},
		{"data:text/html;base64,PHNjcmlwdD4=", ""},
		{"data:,hello", ""},
		{"data:image/,x", ""},
		{"data:image/png", ""},
		{"javascript:alert(1)", ""},
		{"cat.png", "cat.png"},
	} {
		at := ParseMarkdown("![a](" + c.src + ")").AttributesAt(0).Attachment
		if at == nil || at.URL != c.want {
			t.Errorf("Markdown %s: attachment %+v, want URL %q", c.src, at, c.want)
		}
		at = ParseHTML(`<img src="` + c.src + `" alt="a">`).AttributesAt(0).Attachment
		if at == nil || at.URL != c.want {
			t.Errorf("HTML %s: attachment %+v, want URL %q", c.src, at, c.want)
		}
		pic := TextOf(Paragraph{Runs: []Run{{Text: string(ObjectReplacement), Attributes: Attributes{Attachment: &Attachment{Name: "a", URL: c.src}}}}})
		if h := pic.HTML(); !strings.Contains(h, `<img src="`+c.want+`"`) {
			t.Errorf("%s written as %s", c.src, h)
		}
		b := encodeClip(pic)
		encodeClip(NewText("another copy")) // the first is read as JSON
		if got, ok := decodeClip(b); !ok || got.AttributesAt(0).Attachment == nil || got.AttributesAt(0).Attachment.URL != c.want {
			t.Errorf("%s copied as %+v", c.src, got.AttributesAt(0).Attachment)
		}
	}
	// A link to a data: URL is still none.
	if l := ParseMarkdown("[x](" + png + ")").AttributesAt(0).Link; l != "" {
		t.Errorf("link %q", l)
	}
	if l := ParseHTML(`<a href="` + png + `">x</a>`).AttributesAt(0).Link; l != "" {
		t.Errorf("HTML link %q", l)
	}
	// Written back as it was read.
	for _, md := range []string{"![a](" + png + ")", "![s](" + svg + " \"t\")", "a ![b](" + png + ") c"} {
		if got := ParseMarkdown(md).Markdown(); got != md {
			t.Errorf("%q written back as %q", md, got)
		}
	}
}

// TestHTMLQuotes checks that quoted lists, headings and code stay quoted
// through HTML.
func TestHTMLQuotes(t *testing.T) {
	in := TextOf(
		Paragraph{Style: ParagraphStyle{Quote: true, Heading: 2}, Runs: []Run{{Text: "QH"}}},
		Paragraph{Style: ParagraphStyle{Quote: true, List: Bullets}, Runs: []Run{{Text: "item"}}},
		Paragraph{Style: ParagraphStyle{Quote: true, CodeBlock: true}, Runs: []Run{{Text: "code"}}},
		Paragraph{Runs: []Run{{Text: "after"}}},
	)
	out := ParseHTML(in.HTML())
	i := 0
	for _, p := range out.Paragraphs() {
		want := i < 3
		i++
		if p.Style.Quote != want {
			t.Errorf("paragraph %d %v: quote %v (%s)", i, p.Runs, p.Style.Quote, in.HTML())
		}
	}
}

// TestRTFWordPad checks what WordPad and Word write: lists of \pntext,
// table cells, hidden text, non-breaking hyphens and links to bookmarks.
func TestRTFWordPad(t *testing.T) {
	at := ParseRTF([]byte(`{\rtf1{\pntext\f1\'B7\tab}{\*\pn\pnlvlblt{\pntxtb\'B7}}one\par{\pntext 1.\tab}{\*\pn\pnlvlbody}first\par
e\_mail vis{\v hid}ible\par alpha\cell beta\cell\row {\field{\*\fldinst HYPERLINK \\l "top"}{\fldrslt up}}\par}`))
	var got []string
	for _, p := range at.Paragraphs() {
		got = append(got, fmt.Sprintf("%d:%s", p.Style.List, plainOf(p)))
	}
	want := []string{"1:one", "2:first", "0:e‑mail visible", "0:alpha\tbeta\t", "0:up"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", got, want)
	}
	if l := at.AttributesAt(at.Len() - 1).Link; l != "#top" {
		t.Errorf("link %q", l)
	}
}
