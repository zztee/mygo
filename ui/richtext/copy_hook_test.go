package richtext

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

func TestCopyHookFormatsAndHistory(t *testing.T) {
	for _, action := range []string{"copy", "cut"} {
		for _, route := range []string{"key", "command", "menu"} {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/reverse=%v", action, route, reverse), func(t *testing.T) {
					raw := "xx" + strings.Repeat("甲🙂b\n", 40) + "zz"
					original := NewText(raw).Format(0, len([]rune(raw)), func(a *Attributes) {
						a.Tag = "unselected source metadata"
					})
					start, end := 2, original.Len()-2
					want := original.Slice(start, end).Format(0, end-start, func(a *Attributes) {
						a.Tag = "selected metadata"
						a.Weight = 700
					}).Append("!", Attributes{})
					d := NewDocument(original)
					calls := 0
					opts := EditorOptions{OnCopy: func(text Text, a, z int) Text {
						calls++
						if !text.Same(original) || a != start || z != end {
							t.Fatalf("snapshot %q range %d,%d", text.String(), a, z)
						}
						return text.Slice(a, z).Format(0, z-a, func(attr *Attributes) {
							attr.Tag = "selected metadata"
							attr.Weight = 700
						}).Append("!", Attributes{})
					}}
					tt, changed, _ := inputHookTester(t, d, &opts)
					a, z := start, end
					if reverse {
						a, z = z, a
					}
					d.SetSelection(a, z)
					tt.Frame()
					*changed = false
					version := d.Version()
					switch route {
					case "key":
						key := ui.KeyC
						if action == "cut" {
							key = ui.KeyX
						}
						tt.Key(ui.Cmd, key)
					case "command":
						tt.Command(action)
					case "menu":
						tt.Key(ui.Shift, ui.KeyF10)
						label := "Copy"
						if action == "cut" {
							label = "Cut"
						}
						chooseEdit(t, tt, label)
					}
					if calls != 1 {
						t.Fatalf("hook calls %d", calls)
					}
					data := tt.ClipboardData()
					for _, tc := range []struct {
						format transfer.Format
						want   string
					}{{transfer.Text, want.PlainText()}, {transfer.HTML, want.HTML()}} {
						b, err := data.Read(tc.format)
						if err != nil || string(b) != tc.want {
							t.Fatalf("%s: %q, %v", tc.format, b, err)
						}
					}
					b, err := data.Read(Format)
					if err != nil {
						t.Fatal(err)
					}
					if got, ok := decodeClip(b); !ok || !got.Equal(want) {
						t.Fatalf("cached clipboard %q, %v", got.String(), ok)
					}
					var clip clipText
					if err := json.Unmarshal(b, &clip); err != nil {
						t.Fatal(err)
					}
					clip.Copy = ""
					b, err = json.Marshal(clip)
					if err != nil {
						t.Fatal(err)
					}
					got, ok := decodeClip(b)
					if !ok || got.String() != want.String() || strings.Contains(string(b), "unselected") {
						t.Fatalf("serialized clipboard: decoded=%v runes=%d", ok, got.Len())
					}
					// The wire format stores runs, without their paragraph marks.
					for i, r := range []rune(got.String()) {
						if r != '\n' && got.AttributesAt(i) != want.AttributesAt(i) {
							t.Fatalf("serialized attributes at rune %d: %+v", i, got.AttributesAt(i))
						}
					}
					if original.String() != raw || original.AttributesAt(start).Tag != "unselected source metadata" || original.AttributesAt(start).Weight != 0 {
						t.Fatal("callback changed its snapshot")
					}
					if action == "copy" {
						if !d.Text().Same(original) || d.Version() != version || *changed || d.CanUndo() {
							t.Fatal("copy changed the document or history")
						}
						if anchor, caret := d.Selection(); anchor != a || caret != z {
							t.Fatalf("copy selection %d,%d", anchor, caret)
						}
					} else {
						if !d.Text().Equal(original.Delete(start, end)) || !*changed {
							t.Fatalf("cut %q", d.Text().String())
						}
						if anchor, caret := d.Selection(); anchor != start || caret != start {
							t.Fatalf("cut selection %d,%d", anchor, caret)
						}
						tt.Command("undo")
						if !d.Text().Equal(original) || d.CanUndo() {
							t.Fatal("cut did not undo in one step")
						}
						if anchor, caret := d.Selection(); anchor != a || caret != z {
							t.Fatalf("undo selection %d,%d", anchor, caret)
						}
					}
				})
			}
		}
	}
}

func TestCopyHookEmptyAndReadOnly(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		t.Run(fmt.Sprint(readonly), func(t *testing.T) {
			d := NewDocument(NewText("text"))
			calls := 0
			opts := EditorOptions{ReadOnly: readonly, OnCopy: func(text Text, start, end int) Text {
				calls++
				return text.Slice(start, end)
			}}
			tt, _, _ := inputHookTester(t, d, &opts)
			tt.SetClipboard("unchanged")
			d.SetSelection(2, 2)
			tt.Frame()
			tt.Command("copy")
			tt.Command("cut")
			if calls != 0 || tt.Clipboard() != "unchanged" {
				t.Fatal("empty selection copied")
			}
			if readonly {
				d.SetSelection(3, 1)
				tt.Frame()
				tt.Command("cut")
				if calls != 0 || tt.Clipboard() != "unchanged" {
					t.Fatal("read-only selection cut")
				}
				tt.Command("copy")
				if calls != 1 || tt.Clipboard() != "ex" {
					t.Fatal("read-only copy did not use the hook")
				}
			}
			if d.Text().String() != "text" || d.CanUndo() {
				t.Fatal("copy changed the document")
			}
		})
	}
}

func TestCopyHookSeesSameBatchInput(t *testing.T) {
	d := NewDocument(NewText("base"))
	calls := 0
	opts := EditorOptions{OnCopy: func(text Text, start, end int) Text {
		calls++
		if text.String() != "Qbase" || start != 0 || end != 5 {
			t.Fatalf("snapshot %q range %d,%d", text.String(), start, end)
		}
		return text.Slice(start, end)
	}}
	tt, _, _ := inputHookTester(t, d, &opts)
	d.SetSelection(0, 0)
	tt.Frame()
	tt.Batch(func() { tt.Type("Q"); tt.Command("selectAll"); tt.Command("copy") })
	if calls != 1 || tt.Clipboard() != "Qbase" || d.Text().String() != "Qbase" {
		t.Fatalf("calls %d clipboard %q text %q", calls, tt.Clipboard(), d.Text().String())
	}
	tt.Command("undo")
	if d.Text().String() != "base" || d.CanUndo() {
		t.Fatal("copy added a history step")
	}
}
