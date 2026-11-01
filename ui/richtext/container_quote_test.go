package richtext

import (
	"github.com/egoist/mygo/ui/markdown/parser"
	"testing"
)

func TestMarkdownListsKeepOuterQuoteDepth(t *testing.T) {
	for _, md := range []string{
		">> - item\n",
		">> 2. first\n>> 3. second\n",
		">> - [x] done\n",
		">> - first\n>>\n>>     second\n",
		">> - one\n>>     - two\n",
		">> - first\n>>\n>>     ```go\n>>     x := 1\n>>     ```\n",
	} {
		text := ParseMarkdown(md)
		for _, p := range text.Paragraphs() {
			if !p.Style.Quote || p.Style.QuoteDepth != 1 {
				t.Fatalf("%q style %+v", md, p.Style)
			}
		}
		a, b := parser.New(parser.GFM), parser.New(parser.GFM)
		a.Set(md)
		b.Set(text.Markdown())
		if got, want := b.HTML(), a.HTML(); got != want {
			t.Fatalf("%q became %q: HTML %q want %q", md, text.Markdown(), got, want)
		}
	}
}
