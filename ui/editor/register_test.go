package editor

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// registered counts the languages tests add, to name each its own.
var registered int

// A language an app adds is found by its name, aliases, extensions and
// file names, after MyGo's own, and colors Markdown's code blocks of it.
func TestRegisterLanguage(t *testing.T) {
	registered++
	name := fmt.Sprintf("moontest%d", registered)
	own := len(Languages())
	moon := &Language{
		Name: "Moonscript " + name, Extensions: []string{"." + name}, Filenames: []string{"Moonfile." + name},
		Aliases: []string{strings.ToUpper(name), "go"}, LineComment: "--",
		Tokenizer: NewLexer(LexerSpec{
			LineComments: []string{"--"}, BlockComments: [][2]string{{"--[[", "]]"}},
			Strings: []LexString{{Open: `"`, Close: `"`, Escape: '\\'}, {Open: "[[", Close: "]]", Multiline: true}},
			Words:   map[string]TokenKind{"if": TokenKeyword, "nil": TokenConstant}, Calls: true, Operators: "=+-",
		}),
	}
	RegisterLanguage(moon)
	langs := Languages()
	if len(langs) != own+1 || langs[own] != moon {
		t.Fatalf("%d languages after %d, the last %v", len(langs), own, langs[len(langs)-1].Name)
	}
	for _, n := range []string{"moonscript " + name, name, strings.ToUpper(name)} {
		if l := LanguageNamed(n); l != moon {
			t.Errorf("LanguageNamed(%q) = %v", n, l)
		}
	}
	if l := LanguageNamed("go"); l == nil || l.Name != "Go" {
		t.Errorf("an alias took Go's name: %v", l)
	}
	for _, file := range []string{"a." + name, "x/Moonfile." + name} {
		if l := LanguageFor(file); l != moon {
			t.Errorf("LanguageFor(%q) = %v", file, l)
		}
	}
	md := LanguageNamed("markdown").Tokenizer
	if got, want := lexLines(md, "```"+name, `if f("a\n") = nil -- c`, "```"),
		`punct(`+"```"+`)type(`+name+`) / kw(if)fn(f)punct(()str("a)esc(\n)str(")punct())op(=)const(nil)comment(-- c) / punct(`+"```"+`)`; got != want {
		t.Errorf("a code block of it:\n got %s\nwant %s", got, want)
	}
}

// NewLexer lexes as the spec says: block comments that nest, strings that
// go on to the next line, and keywords in any case.
func TestNewLexer(t *testing.T) {
	lx := NewLexer(LexerSpec{
		BlockComments: [][2]string{{"(*", "*)"}}, NestComments: true,
		Strings:  []LexString{{Open: "'", Close: "'", Multiline: true}},
		Words:    map[string]TokenKind{"begin": TokenKeyword, "end": TokenKeyword},
		FoldCase: true, Variables: true, Operators: ":=",
	})
	if got, want := lexLines(lx, "BEGIN (* a (* b *) c *) x := 'one", "two'; $y End"),
		"kw(BEGIN)comment((* a (* b *) c *))var(x)op(:=)str('one) / str(two')punct(;)var($y)kw(End)"; got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

// Languages may be added while others look them up.
func TestRegisterLanguageRace(t *testing.T) {
	registered++
	name := fmt.Sprintf("racetest%d", registered)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			if i == 0 {
				RegisterLanguage(&Language{Name: name})
				return
			}
			for range 100 {
				LanguageNamed(name)
				LanguageFor("x.go")
			}
		})
	}
	wg.Wait()
	if l := LanguageNamed(name); l == nil || l.Name != name {
		t.Fatalf("LanguageNamed(%q) = %v", name, l)
	}
}
