package main

/*
	Each grammar needs its own parser and lexer initialization which includes the import of the parser package.
	We do this by searching for 'DO NOT REMOVE THIS LINE' and insert the lines below with a Bash script.

	E.g., for SQLite, we need to insert these subsequent lines:

	parser "atnwalk/out/gen/sqlite"
    "atnwalk"
*/
import (
	// DO NOT REMOVE THIS LINE - IMPORT
	"bufio"
	"fmt"
	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
	"io"
	"os"
)

var _parser_ antlr.Parser
var _lexer antlr.Lexer

func main() {
	reader := bufio.NewReader(os.Stdin)
	var data []byte
	data, _ = io.ReadAll(reader)
	if len(data) == 0 {
		os.Exit(0)
	}

	/*
		Each grammar needs its own parser and lexer initialization.
		We do this by searching for 'DO NOT REMOVE THIS LINE' and insert the lines below with a Bash script.

		E.g., for SQLite, we need to insert these subsequent lines:

		// create the input and token streams, lexer, and parser instances
		input := antlr.NewInputStream(string(data))
		lexer := parser.NewSQLiteLexer(input)
		stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
		parser_ := parser.NewSQLiteParser(stream)

		// call the main rule that shall be parsed
		tree := parser_.Start()

		// create an ATNWalker
		walker := atnwalk.NewATNWalker(parser_, lexer)
		os.Stdout.Write(walker.Encode(tree))

		// this is just a safety measure to make sure that both, lexer and parser, have been properly initialized
		_parser_ = parser_
		_lexer = lexer
	*/

	// DO NOT REMOVE THIS LINE - EXEC

	if _parser_ == nil || _lexer == nil {
		panic(fmt.Errorf("_parser_ or _lexer are nil, make sure to insert the appropriate parser and lexer " +
			"initialization into the code; inspect the comment above this panic statement in the code"))
	}

	os.Exit(0)
}
