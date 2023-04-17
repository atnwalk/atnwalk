package main

import (
	"atnwalk"
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"time"
)

const SocketFile = "./atnwalk.socket"

func main() {
	var data1, data2 []byte
	var seedCrossover, seedMutation uint64
	var err error
	var wanted byte = 0
	timeout := 500
	for i, arg := range os.Args[1:] {
		i += 1
		switch arg {
		case "-c":
			wanted |= atnwalk.CrossoverBit
			if len(os.Args[i+1:]) < 3 {
				panic("Not enough arguments for '-c' option, need: FILE_1 FILE_2 SEED")
			}
			data1, err = ioutil.ReadFile(os.Args[i+1])
			if err != nil {
				panic("Failed to read: " + os.Args[i+1] + " (need: FILE_1)")
			}
			data2, err = ioutil.ReadFile(os.Args[i+2])
			if err != nil {
				panic("Failed to read: " + os.Args[i+2] + " (need: FILE_2)")
			}
			seedCrossover, err = strconv.ParseUint(os.Args[i+3], 10, 64)
			if err != nil {
				panic("Could not parse uint64: " + os.Args[i+3] + " (need: SEED)")
			}
		case "-m":
			wanted |= atnwalk.MutateBit
			if len(os.Args[i+1:]) < 1 {
				panic("Not enough arguments for '-m' option, need: SEED")
			}
			seedMutation, err = strconv.ParseUint(os.Args[i+1], 10, 64)
			if err != nil {
				panic("Could not parse uint64: " + os.Args[i+1] + " (need: SEED)")
			}
		case "-d":
			wanted |= atnwalk.DecodeBit
		case "-e":
			wanted |= atnwalk.EncodeBit
		case "-t":
			timeout, err = strconv.Atoi(os.Args[i+1])
			if err != nil {
				panic("Could not parse int: " + os.Args[i+1] + " (need: TIMEOUT in ms)")
			}
		}
	}

	if data1 == nil {
		reader := bufio.NewReader(os.Stdin)
		data1, err = io.ReadAll(reader)
		if err != nil {
			panic("Could not read from STDIN")
		}
	}
	encoded, decoded := &([]byte{}), &([]byte{})

	for !atnwalk.SendRequest(SocketFile, timeout, data1, data2, wanted, seedCrossover, seedMutation, encoded, decoded) {
		// atnwalk.RestartServer(LockFile, ServerBin)
		time.Sleep(50 * time.Millisecond)
	}

	if wanted&atnwalk.DecodeBit > 0 {
		if decoded == nil || len(*decoded) == 0 {
			os.Stdout.Write([]byte{0})
		} else {
			os.Stdout.Write(*decoded)
		}
	}

	if wanted&atnwalk.CrossoverBit > 0 || wanted&atnwalk.MutateBit > 0 || wanted&atnwalk.EncodeBit > 0 {
		if encoded == nil || len(*encoded) == 0 {
			os.Stderr.Write([]byte{0})
		} else {
			os.Stderr.Write(*encoded)
		}
	}
}
