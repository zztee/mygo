package editor

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Wrapped CJK text keeps the line breaking rules of JIS X 4051 and of
// Chinese typesetting (kinsoku): no row starts with closing punctuation,
// as 。 or 」, and none ends with opening punctuation, as 「; the character
// before goes down with it instead.
func TestWrapKinsoku(t *testing.T) {
	v := wrapView(t)
	const noStart = "、。，．：；？！）〕］｝〉》」』】…‥ー々ぁっゃァッャ・”’.,;:?!)]}"
	const noEnd = "（〔［｛〈《「『【“‘([{"
	texts := []string{
		"中文中文。下一句话，还有顿号、分号；冒号：问号？叹号！最后一句。",
		"他说「你好」然后『再见』，见【注一】〔甲〕《书名》〈篇名〉（括号）的说明。",
		"省略号……还有两点‥以及长音ラーメンとカレー。ちょっと待ってください、キャッシュ。人々・中点。",
		"中文.混排, 半角; 标点: 问号? 叹号! (括号) [方括号] {花括号}后面。",
		"“引号”和‘单引号’都在这里，“再来一个”。",
	}
	cjk := v.widths.of('中')
	if cjk <= 0 {
		t.Fatal("中 has no width")
	}
	for _, s := range texts {
		for cols := 4; cols <= 12; cols++ {
			v.wrapWidth = cjk*float32(cols) + cjk/2
			breaks := v.wrapBreaks(s)
			if len(breaks) == 0 {
				t.Fatalf("%q did not wrap at %d columns", s, cols)
			}
			from := 0
			for _, at := range append(breaks, len(s)) {
				row := s[from:at]
				first, _ := utf8.DecodeRuneInString(row)
				last, _ := utf8.DecodeLastRuneInString(row)
				if from > 0 && strings.ContainsRune(noStart, first) {
					t.Errorf("%d columns: row %q starts with %q", cols, row, first)
				}
				if at < len(s) && strings.ContainsRune(noEnd, last) {
					t.Errorf("%d columns: row %q ends with %q", cols, row, last)
				}
				from = at
			}
		}
	}
}

// Kinsoku only moves where CJK text and punctuation break: code breaks
// where it did.
func TestCanBreak(t *testing.T) {
	for _, c := range []struct {
		prev, r rune
		want    bool
	}{
		{' ', 'x', true}, {'x', ' ', false}, {'a', '(', true}, {'(', 'a', true}, {'a', '.', false},
		{')', 'a', true}, {'a', 'b', false}, {'中', '文', true}, {'中', 'a', true}, {'a', '中', true},
		{'文', '。', false}, {'文', '，', false}, {'文', '」', false}, {'「', '文', false}, {'文', '「', true},
		{'。', '下', true}, {'」', '下', true}, {'文', '.', false}, {'(', '注', false}, {'文', '(', true},
		{'文', '…', false}, {'文', 'ー', false}, {'文', '”', false}, {'“', '文', false}, {' ', '。', false},
		{'o', '”', false}, {'“', 'h', false},
	} {
		if got := canBreak(c.prev, c.r); got != c.want {
			t.Errorf("canBreak(%q, %q) = %v, want %v", c.prev, c.r, got, c.want)
		}
	}
}
