package richtext

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func codeNumberText(n int) Text {
	paras := make([]Paragraph, n)
	for i := range paras {
		paras[i] = Paragraph{Style: ParagraphStyle{CodeBlock: true, Language: "go"}, Runs: []Run{{Text: fmt.Sprintf("x%d := \"line %d\" /* note */", i, i)}}}
	}
	return TextOf(paras...)
}

func BenchmarkCodeNumbersIdle(b *testing.B) {
	for _, on := range []bool{false, true} {
		b.Run(fmt.Sprint(on), func(b *testing.B) {
			for _, n := range []int{1000, 10000} {
				b.Run(fmt.Sprint(n), func(b *testing.B) {
					doc := NewDocument(codeNumberText(n))
					tt := ui.NewTester(func(c *ui.Context) { Editor(c, doc, EditorOptions{CodeLineNumbers: on}).Fill() }, 1000, 800)
					tt.Frame()
					tt.Frame()
					b.ReportAllocs()
					for b.Loop() {
						tt.Frame()
					}
				})
			}
		})
	}
}
