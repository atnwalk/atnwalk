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
	"fmt"
	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
	"net"
	"os"
	"runtime"
	"strconv"
)

const (
	PidFile    = "./atnwalk.pid"
	SocketFile = "./atnwalk.socket"
)

var parser_ antlr.Parser
var lexer antlr.Lexer

func main() {
	atnwalk.InitServerProcess(PidFile, SocketFile)

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

	if err := os.RemoveAll(SocketFile); err != nil {
		panic(err)
	}

	timeout := 500
	if len(os.Args) > 1 {
		var err error
		if timeout, err = strconv.Atoi(os.Args[1]); err != nil {
			panic(err)
		}
	}

	listener, err := net.Listen("unix", SocketFile)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	semaphore := make(chan struct{}, runtime.NumCPU())
	for {
		semaphore <- struct{}{}
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}
		go func() {
			atnwalk.HandleRequest(conn, timeout, parser_, lexer)
			<-semaphore
		}()
	}
}
