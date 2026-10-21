package editor

// RegisterLanguage adds l to the languages MyGo knows, after its own and
// those added before: LanguageFor and LanguageNamed find it, and
// Markdown's code blocks of it are highlighted by its Tokenizer. Where a
// name or an extension is a language's that comes before, that one wins.
// Apps add their languages once, before they are used, as from an init
// function; adding them is safe at any time.
func RegisterLanguage(l *Language) {
	languagesOnce.Do(makeLanguages)
	languagesMu.Lock()
	defer languagesMu.Unlock()
	old := *languageList.Load()
	list := append(old[:len(old):len(old)], l)
	languageList.Store(&list)
}

// LexerSpec describes the tokens of a language of the kind most code is
// in, for NewLexer: identifiers and keywords, numbers, strings with
// escapes, line and block comments, operators and punctuation.
type LexerSpec struct {
	// LineComments start comments to the end of the line, as "//", and
	// BlockComments enclose comments, as {"/*", "*/"}: at most 31. With
	// NestComments, a block comment holds others.
	LineComments  []string
	BlockComments [][2]string
	NestComments  bool
	// Strings are its kinds of string literal, at most 31.
	Strings []LexString
	// Words gives the kinds of the identifiers that are keywords, types,
	// constants or functions, looked up in lower case with FoldCase, as
	// SQL's.
	Words    map[string]TokenKind
	FoldCase bool
	// IdentChars are characters other than letters, digits and _ that
	// identifiers hold, as $ in JavaScript.
	IdentChars string
	// CapitalTypes makes capitalized identifiers types, Calls those
	// before ( functions, and Keys those and strings before : properties,
	// as JSON's.
	CapitalTypes, Calls, Keys bool
	// Decorator starts annotations, as @ in Java and Python.
	Decorator byte
	// Directives makes a # first on a line a preprocessor line, as C's.
	Directives bool
	// Regexps lexes /.../ where an expression may start, as JavaScript
	// does, and Variables $name as a variable, as shells and PHP do.
	Regexps, Variables bool
	// Operators are the characters of operators.
	Operators string
}

// LexString is a kind of string literal.
type LexString struct {
	// Open and Close enclose the string.
	Open, Close string
	// Escape starts escape sequences, as \; 0 for raw strings.
	Escape byte
	// Multiline strings go on past the end of a line.
	Multiline bool
}

// NewLexer returns a Tokenizer of the tokens spec describes, as MyGo's
// own languages have, for a Language to RegisterLanguage.
func NewLexer(spec LexerSpec) Tokenizer {
	if len(spec.BlockComments) > 31 || len(spec.Strings) > 31 {
		panic("editor: a LexerSpec with more than 31 block comments or strings")
	}
	lx := &lexer{
		lineComments: spec.LineComments, blockComments: spec.BlockComments, nestComments: spec.NestComments,
		words: spec.Words, foldCase: spec.FoldCase, identChars: spec.IdentChars,
		capitalTypes: spec.CapitalTypes, calls: spec.Calls, keys: spec.Keys, decorator: spec.Decorator,
		directives: spec.Directives, regexps: spec.Regexps, variables: spec.Variables, operators: spec.Operators,
	}
	for _, s := range spec.Strings {
		lx.strings = append(lx.strings, lexString{open: s.Open, close: s.Close, escape: s.Escape, multiline: s.Multiline, kind: TokenString})
	}
	return lx
}
