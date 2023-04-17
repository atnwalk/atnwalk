package main

import (
	"atnwalk"
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"time"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	var data []byte

	if len(os.Args) >= 4 && os.Args[1] == "-c" {
		data1, _ := ioutil.ReadFile(os.Args[2])
		data2, _ := ioutil.ReadFile(os.Args[3])
		data = atnwalk.Crossover(data1, data2, time.Now().Unix())
	} else {
		data, _ = io.ReadAll(reader)
		if len(data) == 0 {
			os.Exit(0)
		}
	}
	os.Stdout.Write(atnwalk.Mutate(data, time.Now().Unix()))
	os.Exit(0)
}
