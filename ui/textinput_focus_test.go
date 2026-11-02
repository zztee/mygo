package ui_test

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"testing"
)

func TestTextKeyFocusSameBatch(t *testing.T) {
	d := richtext.NewDocument(richtext.NewText("body"))
	var body ui.Handle
	cell, show := "cell", true
	var committed string
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			richtext.EditorBase(c, d, richtext.EditorOptions{Label: "Body"}).Fill().Bind(&body)
			if show {
				ui.TextArea(c, &cell).Label("Cell").OnTextKey(func(k ui.TextKey) bool {
					if k.Key != ui.KeyBackspace || k.Mods != ui.Cmd|ui.Shift {
						return false
					}
					committed = k.Text()
					d.SetSelection(0, 0)
					if !k.Focus(body) {
						t.Fatal("body did not take immediate focus")
					}
					show = false
					return true
				})
			}
		})
	}, 500, 400)
	if err := tt.Click("Cell"); err != nil {
		t.Fatal(err)
	}
	tt.Batch(func() {
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("edited")
		tt.Key(ui.Cmd|ui.Shift, ui.KeyBackspace)
		tt.Type("fresh")
	})
	if committed != "edited" || d.Text().String() != "freshbody" {
		t.Fatalf("cell %q body %q", committed, d.Text().String())
	}
	tt.Command("undo")
	if d.Text().String() != "body" {
		t.Fatalf("undo %q", d.Text().String())
	}
}

func TestTextKeyFocusUnavailableAndLifetime(t *testing.T) {
	for _, mode := range []string{"hidden", "disabled", "nonfocusable", "invisible"} {
		t.Run(mode, func(t *testing.T) {
			var target ui.Handle
			var saved ui.TextKey
			text, show := "", true
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Fill().Children(func() {
					if show {
						if mode == "nonfocusable" {
							ui.Text(c, "Target").Bind(&target)
						} else {
							in := ui.TextInput(c, new(string)).Label("Target").Disabled(mode == "disabled").Bind(&target)
							if mode == "invisible" {
								in.Invisible()
							}
						}
					}
					ui.TextInput(c, &text).Label("Cell").OnTextKey(func(k ui.TextKey) bool {
						if k.Key != ui.KeyEnter {
							return false
						}
						saved = k
						if k.Focus(target) {
							t.Fatal("unavailable target took focus")
						}
						return true
					})
				})
			}, 400, 200)
			if mode == "hidden" {
				show = false
				tt.Frame()
			}
			if err := tt.Click("Cell"); err != nil {
				t.Fatal(err)
			}
			tt.Batch(func() { tt.Key(0, ui.KeyEnter); tt.Type("kept") })
			if text != "kept" {
				t.Fatalf("text %q", text)
			}
			defer func() {
				if recover() == nil {
					t.Error("expired TextKey.Focus did not panic")
				}
			}()
			saved.Focus(target)
		})
	}
}
