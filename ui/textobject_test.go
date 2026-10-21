package ui

import "testing"

// TestTextObjects lays out an inline object between words: it takes its
// width in the line, stands its height on the baseline, and lines break
// around it.
func TestTextObjects(t *testing.T) {
	font := Font{Size: 14}
	plain := ShapeRichText([]Span{{Text: "ab"}}, font, 0)
	pw, ph := plain.Size()
	l := ShapeRichText([]Span{{Text: "a"}, {Text: "￼", ObjectWidth: 40, ObjectHeight: 50}, {Text: "b"}}, font, 0)
	w, h := l.Size()
	if w < pw+39 || w > pw+41 {
		t.Errorf("width %v, want %v and the object's 40", w, pw)
	}
	if h < 50 || h <= ph {
		t.Errorf("height %v: the line does not hold the object", h)
	}
	objects := l.Objects()
	if len(objects) != 1 || objects[0].Index != 1 {
		t.Fatalf("objects %+v", objects)
	}
	r := objects[0].Rect
	if r.W < 39 || r.W > 41 || r.H != 50 || r.X <= 0 || r.Y < 0 {
		t.Errorf("object box %+v", r)
	}
	// The caret after it is past its box.
	if c := l.Caret(2); c.X < r.X+r.W-1 {
		t.Errorf("caret after the object at %v, box %+v", c.X, r)
	}
	// Narrow, the line breaks around it rather than overflowing.
	narrow := ShapeRichText([]Span{{Text: "words words "}, {Text: "￼", ObjectWidth: 60, ObjectHeight: 20}, {Text: " more"}}, font, 90)
	if _, nh := narrow.Size(); nh < 2*20 {
		t.Errorf("narrow height %v: no line broke at the object", nh)
	}
}

// TestTextObjectDescent hangs an object's descent below the baseline, as a
// formula's depth: the line rises less and drops more than for the same
// box standing on the baseline.
func TestTextObjectDescent(t *testing.T) {
	font := Font{Size: 14}
	box := func(descent float32) (Rect, float32) {
		l := ShapeRichText([]Span{{Text: "a"}, {Text: "￼", ObjectWidth: 40, ObjectHeight: 50, ObjectDescent: descent}, {Text: "b"}}, font, 0)
		objects := l.Objects()
		if len(objects) != 1 {
			t.Fatalf("objects %+v", objects)
		}
		_, h := l.Size()
		return objects[0].Rect, h
	}
	standing, sh := box(0)
	hanging, hh := box(20)
	if hanging.H != 50 || standing.H != 50 {
		t.Fatalf("boxes %+v %+v", standing, hanging)
	}
	// Standing, the box's bottom is the baseline, above the text's
	// descent; hanging 20 below it, the box fills the line, which is as
	// tall as the box, shorter than with the box standing.
	if standing.Y+standing.H >= sh {
		t.Errorf("standing box %+v in a line %v tall", standing, sh)
	}
	if hanging.Y > 0.5 || hanging.Y+hanging.H < hh-0.5 || hh >= sh {
		t.Errorf("hanging box %+v in a line %v tall, %v standing", hanging, hh, sh)
	}
}
