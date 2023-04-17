package main

/*
	Each grammar needs its own parser and lexer initialization which includes the import of the parser package.
	We do this by searching for 'DO NOT REMOVE THIS LINE' and insert the lines below with a Bash script.

	E.g., for SQLite, we need to insert these subsequent lines:

	parser "atnwalk/out/gen/sqlite"
*/
import (
	// DO NOT REMOVE THIS LINE - IMPORT
	"atnwalk"
	"bufio"
	"fmt"
	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
	"io"
	"os"
	"time"
)

var parser_ antlr.Parser
var lexer antlr.Lexer

func main() {

	reader := bufio.NewReader(os.Stdin)
	var data []byte
	data, _ = io.ReadAll(reader)
	if len(data) == 0 {
		os.Exit(0)
	}

	var writeBack *[]byte
	if len(os.Args) > 1 && os.Args[1] == "-wb" {
		writeBack = &([]byte{})
	}

	go func() {
		<-time.After(400 * time.Millisecond)
		os.Exit(0)
	}()

	/*
		Each grammar needs its own parser and lexer initialization.
		We do this by searching for 'DO NOT REMOVE THIS LINE' and insert the lines below with a Bash script.

		E.g., for SQLite, we need to insert these subsequent lines:

		parser_ = parser.NewSQLiteParser(nil)
		lexer = parser.NewSQLiteLexer(nil)
	*/

	// DO NOT REMOVE THIS LINE - EXEC

	if parser_ == nil || lexer == nil {
		panic(fmt.Errorf("parser_ or lexer are nil, make sure to insert the appropriate parser and lexer " +
			"initialization into the code; inspect the comment above this panic statement in the code"))
	}

	walker := atnwalk.NewATNWalker(parser_, lexer)
	output := walker.Decode(data, writeBack)
	os.Stdout.WriteString(output)
	if writeBack != nil {
		os.Stderr.Write(*writeBack)
	}

	os.Exit(0)
}
