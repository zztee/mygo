package editor

// Extra languages are the ones Typora colors beyond the first set.
// The markdown highlighter stores a language index in eight bits, so the
// whole list stays under 256.

var bracePairs = []BracketPair{{Open: "(", Close: ")"}, {Open: "[", Close: "]"}, {Open: "{", Close: "}"}}

func extraLanguages() []*Language {
	quotes := []lexString{cString("\""), cString("'")}
	cLike := func(name string, ext []string, kw, types, cons string) *Language {
		return &Language{
			Name: name, Extensions: ext, LineComment: "//", BlockComment: [2]string{"/*", "*/"},
			Brackets: bracePairs, AutoClose: cAutoClose("\"", "'"), TabSize: 2,
			Tokenizer: &lexer{lineComments: []string{"//"}, blockComments: [][2]string{{"/*", "*/"}}, strings: quotes,
				words: words(TokenKeyword, kw, TokenType, types, TokenConstant, cons),
				calls: true, capitalTypes: true, operators: cOperators},
		}
	}
	hash := func(name string, ext []string, kw, types, cons string, fold bool) *Language {
		return &Language{
			Name: name, Extensions: ext, LineComment: "#",
			Brackets: bracePairs, AutoClose: cAutoClose("\"", "'"), TabSize: 2,
			Tokenizer: &lexer{lineComments: []string{"#"}, strings: quotes, foldCase: fold,
				words: words(TokenKeyword, kw, TokenType, types, TokenConstant, cons),
				calls: true, operators: cOperators},
		}
	}
	dart := cLike("Dart", []string{".dart"},
		"assert async await break case catch class const continue default do else enum extends extension final finally for if in implements import is late library new on part required rethrow return super switch this throw try typedef var void while with yield abstract external factory get operator set static covariant show hide as mixin sealed base interface",
		"int double num bool String List Map Set Future Stream Object dynamic Never void",
		"true false null")
	scala := cLike("Scala", []string{".scala", ".sc"},
		"abstract case catch class def do else extends final finally for if implicit import lazy match new object override package private protected return sealed super this throw trait try type val var while with yield given using enum export",
		"Any AnyRef AnyVal Boolean Byte Char Double Float Int Long Short String Unit Nothing Option List Map Set Seq Array",
		"true false null")
	groovy := cLike("Groovy", []string{".groovy", ".gradle", ".gvy"},
		"abstract as assert break case catch class const continue def default do else enum extends final finally for goto if implements import in instanceof interface native new package private protected public return static super switch synchronized this throw throws trait transient try var void volatile while yield",
		"boolean byte char double float int long short void String Object Integer",
		"true false null")
	solidity := cLike("Solidity", []string{".sol"},
		"abstract as assembly break catch constant constructor continue contract default delete do else emit enum error event external fallback for function if immutable import indexed interface internal is library mapping memory modifier new override payable pragma private public pure receive return returns revert storage struct try type unchecked using view virtual while",
		"address bool string byte bytes int uint uint8 uint256 int256 bytes32",
		"true false")
	dlang := cLike("D", []string{".d"},
		"abstract alias align asm assert auto break case cast catch class const continue default delegate do else enum export extern final finally for foreach function goto if immutable import in inout interface invariant is lazy mixin module new nothrow out override package pragma private protected public pure ref return scope shared static struct super switch synchronized template this throw try typeid typeof union unittest version void while with",
		"string wstring dstring size_t ptrdiff_t bool byte char int long float double",
		"true false null")
	haskell := &Language{
		Name: "Haskell", Extensions: []string{".hs", ".lhs"}, LineComment: "--", BlockComment: [2]string{"{-", "-}"},
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 2,
		Tokenizer: &lexer{lineComments: []string{"--"}, blockComments: [][2]string{{"{-", "-}"}}, nestComments: true,
			strings: []lexString{cString("\"")},
			words: words(TokenKeyword, "case class data default deriving do else foreign hiding if import in infix infixl infixr instance let module newtype of qualified then type where",
				TokenType, "Int Integer Float Double Bool Char String Maybe Either IO",
				TokenConstant, "True False Nothing Just"),
			calls: true, capitalTypes: true, operators: cOperators},
	}
	elm := &Language{
		Name: "Elm", Extensions: []string{".elm"}, LineComment: "--", BlockComment: [2]string{"{-", "-}"},
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 4,
		Tokenizer: &lexer{lineComments: []string{"--"}, blockComments: [][2]string{{"{-", "-}"}}, nestComments: true,
			strings: []lexString{cString("\"")},
			words: words(TokenKeyword, "if then else case of let in type alias module import exposing as port where",
				TokenType, "Int Float Bool String List Maybe Result",
				TokenConstant, "True False Nothing Just"),
			calls: true, capitalTypes: true, operators: cOperators},
	}
	ocaml := &Language{
		Name: "OCaml", Extensions: []string{".ml", ".mli"}, BlockComment: [2]string{"(*", "*)"},
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 2,
		Tokenizer: &lexer{blockComments: [][2]string{{"(*", "*)"}}, nestComments: true, strings: []lexString{cString("\"")},
			words: words(TokenKeyword, "and as assert begin class constraint do done downto else end exception external false for fun function functor if in include inherit let match method mod module mutable new object of open or private rec sig struct then to true try type val virtual when while with",
				TokenType, "int float bool char string list option array unit",
				TokenConstant, "true false"),
			calls: true, capitalTypes: true, operators: cOperators},
	}
	fsharp := &Language{
		Name: "F#", Extensions: []string{".fs", ".fsi", ".fsx"}, LineComment: "//", BlockComment: [2]string{"(*", "*)"},
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 4,
		Tokenizer: &lexer{lineComments: []string{"//"}, blockComments: [][2]string{{"(*", "*)"}}, nestComments: true, strings: quotes,
			words: words(TokenKeyword, "abstract and as assert base begin class default do done else end exception extern false finally for fun function if in inherit interface internal lazy let match member module mutable namespace new null of open or override private public rec return static struct then to true try type use val void when while with yield",
				TokenType, "int float bool char string list option array unit",
				TokenConstant, "true false null"),
			calls: true, capitalTypes: true, operators: cOperators},
	}
	erlang := &Language{
		Name: "Erlang", Extensions: []string{".erl", ".hrl"}, LineComment: "%",
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 4,
		Tokenizer: &lexer{lineComments: []string{"%"}, strings: []lexString{cString("\"")},
			words: words(TokenKeyword, "after and andalso band begin bnot bor bsl bsr bxor case catch cond div end fun if let not of or orelse receive rem try when xor",
				TokenConstant, "true false undefined"),
			calls: true, operators: cOperators},
	}
	clojure := &Language{
		Name: "Clojure", Extensions: []string{".clj", ".cljs", ".cljc", ".edn"}, LineComment: ";",
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 2,
		Tokenizer: &lexer{lineComments: []string{";"}, strings: []lexString{cString("\"")},
			words: words(TokenKeyword, "def defn defmacro defmethod defmulti defprotocol defrecord deftype defonce ns import require refer use fn let loop if when when-not cond case do doseq for recur throw try catch finally new and or not",
				TokenConstant, "true false nil"),
			operators: cOperators},
	}
	lisp := &Language{
		Name: "Lisp", Extensions: []string{".lisp", ".lsp", ".cl"}, LineComment: ";",
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 2,
		Tokenizer: &lexer{lineComments: []string{";"}, strings: []lexString{cString("\"")},
			words: words(TokenKeyword, "defun defmacro defvar defparameter defconstant defclass defmethod defstruct defpackage let lambda if when unless cond case loop do dolist progn return quote setq setf and or not",
				TokenConstant, "t nil"),
			operators: cOperators},
	}
	scheme := &Language{
		Name: "Scheme", Extensions: []string{".scm", ".ss", ".rkt"}, LineComment: ";",
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 2,
		Tokenizer: &lexer{lineComments: []string{";"}, strings: []lexString{cString("\"")},
			words: words(TokenKeyword, "define lambda let let* letrec if cond case and or not begin do quote set!",
				TokenConstant, "true false"),
			operators: cOperators},
	}
	powershell := &Language{
		Name: "PowerShell", Extensions: []string{".ps1", ".psm1", ".psd1"}, LineComment: "#", BlockComment: [2]string{"<#", "#>"},
		Brackets: bracePairs, AutoClose: cAutoClose("\"", "'"), TabSize: 4,
		Tokenizer: &lexer{lineComments: []string{"#"}, blockComments: [][2]string{{"<#", "#>"}}, foldCase: true, variables: true, strings: quotes,
			words: words(TokenKeyword, "begin break catch class continue data do dynamicparam else elseif end exit filter finally for foreach from function if in param process return switch throw trap try until using while",
				TokenConstant, "true false null"),
			calls: true, operators: cOperators},
	}
	latex := &Language{
		Name: "LaTeX", Extensions: []string{".tex", ".latex", ".ltx", ".sty", ".cls"}, LineComment: "%",
		Brackets: bracePairs, TabSize: 2,
		Tokenizer: &lexer{lineComments: []string{"%"}, identChars: `\`,
			words:     words(TokenKeyword, `\documentclass \usepackage \begin \end \section \subsection \subsubsection \chapter \part \paragraph \textbf \textit \emph \texttt \item \label \ref \cite \footnote \caption \includegraphics \title \author \maketitle \newcommand \renewcommand \frac \sqrt \sum \int \text`),
			operators: "{}[]&$#^_"},
	}
	matlab := &Language{
		Name: "MATLAB", Extensions: []string{".matlab"}, LineComment: "%", BlockComment: [2]string{"%{", "%}"},
		Brackets: bracePairs, AutoClose: cAutoClose("'"), TabSize: 4,
		Tokenizer: &lexer{lineComments: []string{"%"}, blockComments: [][2]string{{"%{", "%}"}},
			strings: []lexString{{open: "'", close: "'", kind: TokenString}, cString("\"")},
			words:   words(TokenKeyword, "break case catch classdef continue else elseif end for function global if otherwise parfor persistent return switch try while", TokenConstant, "true false"),
			calls:   true, operators: cOperators},
	}
	pascal := &Language{
		Name: "Pascal", Extensions: []string{".pas", ".pp", ".dpr"}, LineComment: "//", BlockComment: [2]string{"(*", "*)"},
		Brackets: bracePairs, AutoClose: cAutoClose("'"), TabSize: 2,
		Tokenizer: &lexer{lineComments: []string{"//"}, blockComments: [][2]string{{"(*", "*)"}}, foldCase: true,
			strings: []lexString{{open: "'", close: "'", kind: TokenString}},
			words:   words(TokenKeyword, "and array begin case const div do downto else end file for function goto if in label mod nil not of or packed procedure program record repeat set then to type until var while with", TokenType, "integer real boolean char string byte word longint", TokenConstant, "true false nil"),
			calls:   true, operators: cOperators},
	}
	vb := &Language{
		Name: "Visual Basic", Extensions: []string{".vb", ".vbs", ".bas"}, LineComment: "'",
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 4,
		Tokenizer: &lexer{lineComments: []string{"'"}, foldCase: true, strings: []lexString{cString("\"")},
			words: words(TokenKeyword, "and as boolean byref byte byval call case catch class const continue dim do each else elseif end enum exit for function get goto if implements imports in inherits integer interface is loop me mod module new next not nothing object of on or private property protected public return select set shared short single static step stop string structure sub then throw to true try typeof until while with xor",
				TokenType, "Boolean Byte Char Date Decimal Double Integer Long Object String",
				TokenConstant, "True False Nothing"),
			calls: true, operators: cOperators},
	}
	vhdl := &Language{
		Name: "VHDL", Extensions: []string{".vhd", ".vhdl"}, LineComment: "--",
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 2,
		Tokenizer: &lexer{lineComments: []string{"--"}, foldCase: true, strings: quotes,
			words: words(TokenKeyword, "abs after alias all and architecture array assert attribute begin block body buffer bus case component configuration constant downto else elsif end entity exit file for function generate generic if in inout is library loop map mod nand new next nor not null of on open or others out package port procedure process range record report return select signal subtype then to type until use variable wait when while with xnor xor",
				TokenType, "bit bit_vector boolean integer natural positive real std_logic std_logic_vector signed unsigned string time character",
				TokenConstant, "true false"),
			calls: true, operators: cOperators},
	}
	fortran := &Language{
		Name: "Fortran", Extensions: []string{".f", ".f90", ".f95", ".f03", ".for"}, LineComment: "!",
		Brackets: bracePairs, AutoClose: cAutoClose("\""), TabSize: 2,
		Tokenizer: &lexer{lineComments: []string{"!"}, foldCase: true, strings: quotes,
			words: words(TokenKeyword, "program end module subroutine function contains use implicit none integer real complex character logical dimension allocatable intent if then else elseif do while select case call return stop continue cycle exit interface type class associate block procedure allocate deallocate",
				TokenType, "integer real double complex character logical",
				TokenConstant, "true false"),
			calls: true, operators: cOperators},
	}
	perl := hash("Perl", []string{".pl", ".pm", ".t"},
		"if else elsif unless while until for foreach do sub package use require my our local return last next redo goto die warn print say given when default",
		"", "true false undef", false)
	perl.Tokenizer.(*lexer).variables = true
	coffeescript := hash("CoffeeScript", []string{".coffee"},
		"break by catch class continue do else extends finally for if in isnt loop new of off on return switch then throw try unless until when while yes no not and or is await",
		"", "true false null", false)
	elixir := hash("Elixir", []string{".ex", ".exs"},
		"alias and after begin case catch cond def defp defmodule defstruct do else end fn for if import in not or quote raise receive require rescue super try unless use when with",
		"", "true false nil", false)
	rlang := hash("R", []string{".r", ".R"},
		"if else for while in function repeat next break return library require source",
		"", "TRUE FALSE NULL NA Inf NaN", false)
	julia := hash("Julia", []string{".jl"},
		"begin break catch const continue do else elseif end export false finally for function global if import let local macro module quote return struct true try using while where",
		"Int Float64 Bool Char String Nothing Any",
		"true false nothing missing", false)
	nginx := hash("Nginx", []string{".nginx"},
		"server location listen root alias index proxy_pass return include upstream http events worker_processes error_log access_log if set rewrite break last",
		"", "", true)
	graphql := hash("GraphQL", []string{".graphql", ".gql"},
		"query mutation subscription fragment on type interface union enum scalar input extend schema directive implements",
		"Int Float String Boolean ID", "true false null", false)
	hcl := hash("HCL", []string{".hcl", ".tf", ".tfvars"},
		"resource data variable output locals module provider terraform for in if",
		"", "true false null", false)
	nix := hash("Nix", []string{".nix"},
		"let in with inherit rec if then else assert or import",
		"", "true false null", false)
	crystal := hash("Crystal", []string{".cr"},
		"abstract alias as begin break case class def do else elsif end ensure enum extend false for fun if include lib macro module next nil of out private protected require rescue return select self struct super then true type typeof union unless until when while yield",
		"Int Float Bool Char String Nil", "true false nil", false)
	nim := hash("Nim", []string{".nim", ".nims"},
		"and as block break case cast const continue defer discard div do elif else end enum except finally for from func if import in include is iterator let macro method mod nil not notin object of or out proc ptr raise ref return template try tuple type using var when while xor yield",
		"int float bool char string", "true false nil", false)
	awk := hash("Awk", []string{".awk"},
		"BEGIN END if else while for in do break continue next exit function return delete print printf getline",
		"", "", false)
	cmake := hash("CMake", []string{".cmake"},
		"if else elseif endif foreach endforeach while endwhile function endfunction macro endmacro set unset include find_package add_executable add_library target_link_libraries cmake_minimum_required project option message return",
		"", "TRUE FALSE ON OFF", true)
	cmake.Filenames = []string{"CMakeLists.txt"}
	verilog := cLike("Verilog", []string{".v", ".vh", ".sv"},
		"always and assign automatic begin case casex casez default else end endcase endfunction endmodule for forever function generate if initial inout input integer module negedge or output parameter posedge reg repeat signed task time wire xor logic always_ff always_comb always_latch",
		"wire reg logic integer real time bit byte int", "")
	batch := &Language{
		Name: "Batch", Extensions: []string{".bat", ".cmd"}, LineComment: "::",
		Brackets: bracePairs, TabSize: 4,
		Tokenizer: &lexer{lineComments: []string{"::", "REM ", "rem "}, foldCase: true, strings: []lexString{cString("\"")},
			words:     words(TokenKeyword, "echo set if else goto call exit for in do exist defined not", TokenConstant, "on off"),
			operators: "=|&<>"},
	}
	assembly := &Language{
		Name: "Assembly", Extensions: []string{".asm", ".s", ".S"}, LineComment: ";",
		Brackets: bracePairs, TabSize: 8,
		Tokenizer: &lexer{lineComments: []string{";", "#", "//"}, strings: quotes,
			words:     words(TokenKeyword, "mov movq movl lea add sub mul div inc dec and or xor not shl shr cmp test jmp je jne jz jnz call ret push pop nop int syscall section global extern", TokenMeta, ".text .data .bss .global .globl"),
			operators: "+-*/=<>#@$%"},
	}
	httpLang := &Language{
		Name: "HTTP", Extensions: []string{".http"}, Brackets: bracePairs, TabSize: 2,
		Tokenizer: &lexer{strings: quotes,
			words: words(TokenKeyword, "GET POST PUT PATCH DELETE HEAD OPTIONS TRACE CONNECT HTTP",
				TokenProperty, "Host Content-Type Content-Length Authorization Accept User-Agent Cookie Set-Cookie Location Cache-Control"),
			operators: ":/"},
	}
	plain := &Language{Name: "Text", Extensions: []string{".txt", ".text"}}
	return []*Language{
		dart, scala, groovy, solidity, dlang, haskell, elm, ocaml, fsharp, erlang,
		clojure, lisp, scheme, powershell, latex, matlab, pascal, vb, vhdl, fortran,
		perl, coffeescript, elixir, rlang, julia, nginx, graphql, hcl, nix, crystal,
		nim, awk, cmake, verilog, batch, assembly, httpLang, plain,
	}
}
