package richtext

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// accessibleText maps the expanded value read by assistive technology to
// the immutable text shown by the editor. Only replaced objects need entries.
type accessibleText struct {
	value   string
	length  int
	objects []accessibleObject
}

type accessibleObject struct {
	source, start, end int
}

func (t Text) accessible() accessibleText {
	raw := t.String()
	m := accessibleText{value: raw, length: t.Len()}
	var out strings.Builder
	bytes, copied, shift := 0, 0, 0
	for start, run := range t.Runs(0, t.Len()) {
		if a := run.Attributes.Attachment; a != nil && a.AccessibleText != "" {
			n := utf8.RuneCountInString(a.AccessibleText)
			i := start
			for b, r := range run.Text {
				if r == ObjectReplacement {
					out.WriteString(raw[copied : bytes+b])
					out.WriteString(a.AccessibleText)
					copied = bytes + b + utf8.RuneLen(r)
					m.objects = append(m.objects, accessibleObject{i, i + shift, i + shift + n})
					shift += n - 1
				}
				i++
			}
		}
		bytes += len(run.Text)
	}
	if len(m.objects) > 0 {
		out.WriteString(raw[copied:])
		m.value = out.String()
		m.length += shift
	}
	return m
}

// offset maps a document boundary to the expanded accessible value.
func (m accessibleText) offset(i int) int {
	k := sort.Search(len(m.objects), func(k int) bool { return m.objects[k].source >= i })
	if k > 0 {
		o := m.objects[k-1]
		i += o.end - o.source - 1
	}
	return i
}

// source maps a boundary back, rounding an interior endpoint out to the
// object's leading or trailing boundary. Exact boundaries remain exact.
func (m accessibleText) source(i int, trailing bool) int {
	i = max(0, min(i, m.length))
	k := sort.Search(len(m.objects), func(k int) bool { return m.objects[k].end > i })
	if k < len(m.objects) {
		o := m.objects[k]
		if i > o.start {
			if trailing {
				return o.source + 1
			}
			return o.source
		}
		return i - (o.start - o.source)
	}
	if k > 0 {
		o := m.objects[k-1]
		return i - (o.end - o.source - 1)
	}
	return i
}

func (m accessibleText) selection(anchor, caret int) (int, int) {
	if anchor == caret {
		i := m.source(anchor, false)
		return i, i
	}
	if anchor < caret {
		return m.source(anchor, false), m.source(caret, true)
	}
	return m.source(anchor, true), m.source(caret, false)
}
