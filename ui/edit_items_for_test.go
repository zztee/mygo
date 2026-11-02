package ui

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/l10n"
)

func TestEditItemsFor(t *testing.T) {
	target := new(int)
	enabled := true
	var commands []string
	tt := coreNewTester(func(c *context) {
		coreBox(c).Fill().Label("editor").ContextMenu(func(m *Menu) {
			m.Submenu("First", func(m *Menu) {
				if name := m.EditItemsFor(target, false, func(string) bool { return enabled }); name != "" {
					commands = append(commands, name)
				}
			})
			m.Submenu("Second", func(m *Menu) {
				if name := m.EditItemsFor("second", true, func(string) bool { return true }); name != "" {
					commands = append(commands, "second:"+name)
				}
			})
		})
	}, 300, 200)
	tt.RightClick("editor")
	if err := tt.ChooseMenuItem("Second", "Copy"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(commands, []string{"second:copy"}) {
		t.Fatalf("commands %q", commands)
	}
	tt.RightClick("editor")
	target = new(int)
	tt.Frame()
	if err := tt.ChooseMenuItem("First", "Cut"); err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 {
		t.Fatalf("stale target %q", commands)
	}
	tt.RightClick("editor")
	enabled = false
	tt.Frame()
	if err := tt.ChooseMenuItem("First", "Cut"); err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 {
		t.Fatalf("stale enabled %q", commands)
	}
	enabled = true
	tt.Frame()
	tt.RightClick("editor")
	if err := tt.ChooseMenuItem("First", "Cut"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(commands, []string{"second:copy", "cut"}) {
		t.Fatalf("commands %q", commands)
	}
}

func TestEditItemsForLocale(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreBox(c).Fill().Label("editor").ContextMenu(func(m *Menu) { m.EditItemsFor("doc", true, func(string) bool { return true }) })
	}, 300, 200)
	tt.SetLocale("zh-Hans")
	tt.Frame()
	tt.RightClick("editor")
	if !slices.Equal(tt.Menu(), []string{l10n.In("zh-Hans").Copy(), "-", l10n.In("zh-Hans").SelectAll()}) {
		t.Fatalf("localized menu %q", tt.Menu())
	}
}

func TestEditItemsForChoiceLifecycle(t *testing.T) {
	for _, mode := range []string{"chosen", "switched", "removed", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			target := new(int)
			visible, disabled := true, false
			commands := 0
			tt := coreNewTester(func(c *context) {
				if !visible {
					coreBox(c).Fill()
					return
				}
				coreBox(c).Fill().Label("editor").Disabled(disabled).ContextMenu(func(m *Menu) {
					if m.EditItemsFor(target, false, func(string) bool { return true }) != "" {
						commands++
					}
				})
			}, 300, 200)
			tt.RightClick("editor")
			switch mode {
			case "switched":
				target = new(int)
			case "removed":
				visible = false
			case "disabled":
				disabled = true
			}
			tt.Frame()
			if err := tt.ChooseMenuItem("Cut"); err != nil {
				t.Fatal(err)
			}
			want := 0
			if mode == "chosen" {
				want = 1
			}
			if commands != want {
				t.Errorf("commands %d, want %d", commands, want)
			}
			if tt.rt.menu.target != nil {
				t.Fatal("choice retained its document")
			}
		})
	}
}
