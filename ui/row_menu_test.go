package ui_test

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestOutlineRowMenu(t *testing.T) {
	chosen, acted := -1, ""
	var s ui.OutlineState[string]
	s.List.Selected = &chosen
	children := func(key string) []string {
		switch key {
		case "folder":
			return []string{"folder/nested"}
		case "folder/nested":
			return []string{"folder/nested/" + strings.Repeat("long", 30)}
		}
		return nil
	}
	s.Open.Add("folder")
	s.Open.Add("folder/nested")
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Outline(c, &s, []string{"folder", "other"}, children, func(key string) {
			ui.Text(c, filepath.Base(key)).SingleLine().Ellipsis("…")
		}).Grow(1).ContextMenu(func(m *ui.Menu) { m.Item("Background") })
	}, 400, 300)
	tt.Frame()
	before := append([]byte(nil), tt.Image().Pix...)
	s.List.RowMenu = func(m *ui.Menu, row int) {
		if m.Item("Act on " + s.Item(row)).Chosen() {
			acted = s.Item(row)
		}
	}
	tt.Frame()
	if !bytes.Equal(before, tt.Image().Pix) {
		t.Fatal("attaching row menus changed the outline's appearance")
	}
	for _, key := range []string{"folder", "folder/nested", "other"} {
		r, ok := tt.Find(filepath.Base(key))
		if !ok {
			t.Fatalf("missing %s", key)
		}
		for _, x := range []float32{r.X - 10, 2, 390, r.X + 2} {
			tt.RightClickAt(x, r.Y+r.H/2)
			if got := tt.Menu(); !slices.Equal(got, []string{"Act on " + key}) {
				t.Fatalf("%s at x=%g: %q", key, x, got)
			}
			if err := tt.ChooseMenuItem("Act on " + key); err != nil {
				t.Fatal(err)
			}
			if acted != key || chosen != -1 || !s.Open.Has("folder/nested") {
				t.Fatalf("acted %q, selected %d, nested open %v", acted, chosen, s.Open.Has("folder/nested"))
			}
		}
	}
	// The row menu also covers the trailing blank space of a long name.
	long := "folder/nested/" + strings.Repeat("long", 30)
	r, ok := tt.Find(filepath.Base(long))
	if !ok {
		t.Fatal("missing long row")
	}
	tt.RightClickAt(390, r.Y+r.H/2)
	if !slices.Equal(tt.Menu(), []string{"Act on " + long}) {
		t.Fatalf("long row: %q", tt.Menu())
	}
	tt.CloseMenu()
	tt.RightClickAt(200, 280)
	if !slices.Equal(tt.Menu(), []string{"Background"}) {
		t.Fatalf("background: %q", tt.Menu())
	}
	tt.CloseMenu()
	// A nil callback restores the container's menu even over an arrow.
	s.List.RowMenu = nil
	tt.Frame()
	r, _ = tt.Find("folder")
	tt.RightClickAt(r.X-10, r.Y+r.H/2)
	if !slices.Equal(tt.Menu(), []string{"Background"}) {
		t.Fatalf("nil callback: %q", tt.Menu())
	}
}

func TestListRowMenuKeyboardAndChildren(t *testing.T) {
	chosen, acted := -1, -1
	var s ui.ListState
	s.Selected = &chosen
	s.RowMenu = func(m *ui.Menu, row int) {
		if m.Item(fmt.Sprintf("Row %d", row)).Chosen() {
			acted = row
		}
	}
	text := "Editable"
	tt := ui.NewTester(func(c *ui.Context) {
		ui.List(c, &s, 3, func(i int) {
			ui.Row(c).Padding(8).Children(func() {
				ui.Text(c, fmt.Sprintf("Item %d", i))
				if i == 1 {
					ui.Button(c, "Child").ContextMenu(func(m *ui.Menu) { m.Item("Child action") })
					ui.TextInput(c, &text).Label("Editor")
				}
			})
		}).Grow(1).ContextMenu(func(m *ui.Menu) { m.Item("Background") })
	}, 450, 300)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyContextMenu)
	if !slices.Equal(tt.Menu(), []string{"Background"}) {
		t.Fatalf("no selection: %q", tt.Menu())
	}
	tt.CloseMenu()
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyContextMenu)
	if !slices.Equal(tt.Menu(), []string{"Row 1"}) {
		t.Fatalf("chosen %d, menu %q", chosen, tt.Menu())
	}
	if err := tt.ChooseMenuItem("Row 1"); err != nil {
		t.Fatal(err)
	}
	if acted != 1 {
		t.Fatalf("acted on %d", acted)
	}
	tt.Key(0, ui.KeyDown)
	tt.Key(ui.Shift, ui.KeyF10)
	if !slices.Equal(tt.Menu(), []string{"Row 2"}) {
		t.Fatalf("Shift+F10 chose %d, menu %q", chosen, tt.Menu())
	}
	tt.CloseMenu()
	if err := tt.RightClick("Child"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tt.Menu(), []string{"Child action"}) {
		t.Fatalf("child menu: %q", tt.Menu())
	}
	tt.CloseMenu()
	if err := tt.Click("Editor"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyContextMenu)
	if got := tt.Menu(); len(got) == 0 || slices.Contains(got, "Row 1") || slices.Contains(got, "Background") {
		t.Fatalf("editor lost its native menu: %q", got)
	}
}

func TestListRowMenuEmptyAndDisabled(t *testing.T) {
	var s ui.ListState
	count, disabled, calls := 0, false, 0
	s.RowMenu = func(m *ui.Menu, row int) { calls++; m.Item("Row") }
	tt := ui.NewTester(func(c *ui.Context) {
		ui.List(c, &s, count, func(i int) { ui.Text(c, "Item").Padding(8) }).
			Grow(1).Disabled(disabled).ContextMenu(func(m *ui.Menu) { m.Item("Background") })
	}, 400, 300)
	tt.Frame()
	tt.RightClickAt(20, 20)
	if !slices.Equal(tt.Menu(), []string{"Background"}) || calls != 0 {
		t.Fatalf("empty list: %q, row calls %d", tt.Menu(), calls)
	}
	tt.CloseMenu()
	count, disabled = 1, true
	tt.Frame()
	tt.RightClickAt(20, 10)
	if calls != 0 || len(tt.Menu()) > 0 {
		t.Fatalf("disabled list: %q, row calls %d", tt.Menu(), calls)
	}
}

// TestOutlineRowMenuSnapshot records the unchanged appearance with the API
// disabled and enabled; menus themselves use the host's native drawing.
func TestOutlineRowMenuSnapshot(t *testing.T) {
	dir := os.Getenv("ROW_MENU_SNAP_DIR")
	if dir == "" {
		return
	}
	var s ui.OutlineState[string]
	for _, phase := range []string{"before", "after"} {
		if phase == "after" {
			s.List.RowMenu = func(m *ui.Menu, row int) { m.Item("Rename") }
		}
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Outline(c, &s, []string{"Notes", "Archive"}, func(string) []string { return []string{} }, func(key string) {
				ui.Text(c, key)
			}).Grow(1)
		}, 400, 200)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, phase+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("encode: %v; close: %v", err, closeErr)
		}
	}
}

func TestTableRowMenuKeyboard(t *testing.T) {
	chosen := -1
	s := ui.ListState{Selected: &chosen, RowMenu: func(m *ui.Menu, row int) { m.Item(fmt.Sprintf("Row %d", row)) }}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Table(c, &s, []ui.TableColumn{{Title: "Name"}}, 2, func(row, col int) {
			ui.Text(c, fmt.Sprintf("Item %d", row))
		}).Grow(1)
	}, 400, 300)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyContextMenu)
	if !slices.Equal(tt.Menu(), []string{"Row 0"}) {
		t.Fatalf("selected %d, table menu: %q", chosen, tt.Menu())
	}
}

func TestListRowMenuFollowsKeys(t *testing.T) {
	items, acted := []string{"First", "Second"}, ""
	s := ui.ListState{}
	s.Key = func(i int) any { return items[i] }
	s.RowMenu = func(m *ui.Menu, row int) {
		if m.Item("Act").Chosen() {
			acted = items[row]
		}
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.List(c, &s, len(items), func(i int) { ui.Text(c, items[i]).Padding(8) }).Grow(1)
	}, 400, 300)
	tt.Frame()
	if err := tt.RightClick("First"); err != nil {
		t.Fatal(err)
	}
	items[0], items[1] = items[1], items[0]
	if err := tt.ChooseMenuItem("Act"); err != nil {
		t.Fatal(err)
	}
	if acted != "First" {
		t.Fatalf("menu followed the old row index: %q", acted)
	}
	acted = ""
	if err := tt.RightClick("First"); err != nil {
		t.Fatal(err)
	}
	items = items[:1]
	if err := tt.ChooseMenuItem("Act"); err != nil {
		t.Fatal(err)
	}
	if acted != "" {
		t.Fatalf("removed row acted on %q", acted)
	}
}

func TestListRowMenuDeletesRows(t *testing.T) {
	for _, count := range []int{1, 3} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			items := []string{"First", "Second", "Third"}[:count]
			s := ui.ListState{RowMenu: func(m *ui.Menu, row int) {
				if m.Item("Delete").Chosen() {
					items = slices.Delete(items, row, row+1)
				}
			}}
			tt := ui.NewTester(func(c *ui.Context) {
				ui.List(c, &s, len(items), func(i int) { ui.Text(c, items[i]).Padding(8) }).Grow(1)
			}, 400, 300)
			tt.Frame()
			if err := tt.RightClick("First"); err != nil {
				t.Fatal(err)
			}
			if err := tt.ChooseMenuItem("Delete"); err != nil {
				t.Fatal(err)
			}
			if len(items) != count-1 || slices.Contains(tt.Texts(), "First") {
				t.Fatalf("delete left %q, showing %q", items, tt.Texts())
			}
		})
	}
}

func TestListRowMenuRevealsChoice(t *testing.T) {
	for _, key := range []ui.Key{ui.KeyContextMenu, ui.KeyF10} {
		t.Run(fmt.Sprint(key), func(t *testing.T) {
			selected, acted := -1, -1
			s := ui.ListState{Selected: &selected, RowMenu: func(m *ui.Menu, row int) {
				if m.Item(fmt.Sprintf("Row %d", row)).Chosen() {
					acted = row
				}
			}}
			tt := ui.NewTester(func(c *ui.Context) {
				ui.List(c, &s, 200, func(i int) { ui.Text(c, fmt.Sprintf("Item %d", i)).Height(24) }).Grow(1)
			}, 400, 200)
			tt.Frame()
			tt.Key(0, ui.KeyTab)
			tt.Key(0, ui.KeyDown)
			s.ScrollTo(100, ui.Start)
			tt.Frame()
			first, _ := s.Visible()
			if first < 90 || slices.Contains(tt.Texts(), "Item 0") {
				t.Fatalf("choice was not scrolled away: %d, %q", first, tt.Texts())
			}
			mods := ui.Modifiers(0)
			if key == ui.KeyF10 {
				mods = ui.Shift
			}
			tt.Key(mods, key)
			if !slices.Equal(tt.Menu(), []string{"Row 0"}) || !slices.Contains(tt.Texts(), "Item 0") {
				t.Fatalf("chosen row not revealed: %q, %q", tt.Menu(), tt.Texts())
			}
			if err := tt.ChooseMenuItem("Row 0"); err != nil {
				t.Fatal(err)
			}
			if acted != 0 {
				t.Fatalf("acted on %d", acted)
			}
		})
	}
}

func TestListRowMenuSynchronizesFrameChanges(t *testing.T) {
	for _, change := range []string{"remove choice", "empty", "reorder"} {
		for _, key := range []ui.Key{ui.KeyContextMenu, ui.KeyF10} {
			t.Run(fmt.Sprint(change, key), func(t *testing.T) {
				items, selected, acted := []string{"First", "Second"}, -1, ""
				s := ui.ListState{Selected: &selected}
				s.Key = func(i int) any { return items[i] }
				s.RowMenu = func(m *ui.Menu, row int) {
					if m.Item(items[row]).Chosen() {
						acted = items[row]
					}
				}
				tt := ui.NewTester(func(c *ui.Context) {
					ui.List(c, &s, len(items), func(i int) { ui.Text(c, items[i]).Height(24) }).Grow(1)
				}, 400, 200)
				tt.Frame()
				tt.Key(0, ui.KeyTab)
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyDown)
				if selected != 1 {
					t.Fatalf("selected %d", selected)
				}
				switch change {
				case "remove choice":
					items = items[:1]
				case "empty":
					items = nil
				case "reorder":
					items[0], items[1] = items[1], items[0]
				}
				mods := ui.Modifiers(0)
				if key == ui.KeyF10 {
					mods = ui.Shift
				}
				// No intervening frame: input still sees the previous layout.
				tt.Key(mods, key)
				if change != "reorder" {
					if len(tt.Menu()) > 0 || acted != "" {
						t.Fatalf("removed choice: menu %q, acted %q", tt.Menu(), acted)
					}
					return
				}
				if !slices.Equal(tt.Menu(), []string{"Second"}) {
					t.Fatalf("menu targeted a different item: %q", tt.Menu())
				}
				if err := tt.ChooseMenuItem("Second"); err != nil {
					t.Fatal(err)
				}
				if acted != "Second" {
					t.Fatalf("acted on %q", acted)
				}
			})
		}
	}
}
