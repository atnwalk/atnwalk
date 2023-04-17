package atnwalk

import (
	"encoding/binary"
	"errors"
	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

const (
	AreYouAlive  byte = 213
	YesIAmAlive  byte = 42
	CrossoverBit byte = 0b00000001
	MutateBit    byte = 0b00000010
	DecodeBit    byte = 0b00000100
	EncodeBit    byte = 0b00001000
)

func readAll(conn net.Conn, data []byte) bool {
	offset := 0
	for offset < len(data) {
		n, err := conn.Read(data[offset:])
		if err != nil {
			return false
		}
		offset += n
	}
	return true
}

func writeAll(conn net.Conn, data []byte) bool {
	offset := 0
	for offset < len(data) {
		n, err := conn.Write(data[offset:])
		if err != nil {
			return false
		}
		offset += n
	}
	return true
}

func HandleRequest(conn net.Conn, timeout int, parser_ antlr.Parser, lexer antlr.Lexer) {
	defer conn.Close()
	buf := make([]byte, 8)
	var result []byte

	// see whether the client knows the secret handshake
	// used to quickly check whether the server is down
	if ok := readAll(conn, buf[:1]); !ok || buf[0] != AreYouAlive {
		return
	}

	// respond a vivid Yes!
	if !writeAll(conn, []byte{YesIAmAlive}) {
		return
	}

	// let's find out what the client wants and how much data1 will be sent
	// prepare the buffer accordingly
	if !readAll(conn, buf[:5]) {
		return
	}
	wanted := buf[0]
	nBytes := int(binary.BigEndian.Uint32(buf[1:]))
	var data1, data2 []byte = make([]byte, nBytes), nil

	// receive the encoded data
	if !readAll(conn, data1) {
		return
	}

	if wanted&CrossoverBit > 0 {
		// we need the other data to cross over with, therefore read how many bytes are required for that
		if !readAll(conn, buf[:4]) {
			return
		}
		nBytes = int(binary.BigEndian.Uint32(buf[:4]))
		data2 = make([]byte, nBytes)

		// obtain the data to crossover with
		if !readAll(conn, data2) {
			return
		}

		// obtain the seed for crossover
		if !readAll(conn, buf[:8]) {
			return
		}
		result = Crossover(data1, data2, int64(binary.BigEndian.Uint64(buf[:8])))
	}

	if wanted&MutateBit > 0 {
		// obtain the seed for mutation
		if !readAll(conn, buf[:8]) {
			return
		}

		// if we performed a crossover then mutate that resulting data otherwise the provided data1
		if wanted&CrossoverBit > 0 {
			result = Mutate(result, int64(binary.BigEndian.Uint64(buf[:8])))
		} else {
			result = Mutate(data1, int64(binary.BigEndian.Uint64(buf[:8])))
		}
	}

	var walker *ATNWalker
	if wanted&DecodeBit > 0 {
		walker = NewATNWalker(parser_, lexer)
		var writeBack *[]byte
		if wanted&EncodeBit > 0 {
			writeBack = &([]byte{})
		}
		walker.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Millisecond))
		if wanted&CrossoverBit > 0 || wanted&MutateBit > 0 {
			result = []byte(walker.Decode(result, writeBack))
		} else {
			result = []byte(walker.Decode(data1, writeBack))
		}

		// send how many bytes decoded data will be sent
		binary.BigEndian.PutUint32(buf[:4], uint32(len(result)))
		if !writeAll(conn, buf[:4]) {
			return
		}

		// send the decoded data
		if !writeAll(conn, result) {
			return
		}

		nBytes = 0
		if writeBack != nil {
			nBytes = len(*writeBack)
		}

		// send how many bytes encoded data will be sent
		binary.BigEndian.PutUint32(buf[:4], uint32(nBytes))
		if !writeAll(conn, buf[:4]) {
			return
		}

		// send the encoded data
		if nBytes > 0 {
			if !writeAll(conn, *writeBack) {
				return
			}
		}

		// early return to not encode again if the bit is set
		return
	}

	if wanted&EncodeBit > 0 {
		// the client only wants the encoded data, i.e., repair the data
		walker = NewATNWalker(parser_, lexer)
		walker.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Millisecond))
		if wanted&CrossoverBit > 0 || wanted&MutateBit > 0 {
			result = walker.Repair(result)
		} else {
			result = walker.Repair(data1)
		}

		// send how many bytes encoded (repaired) data will be sent
		binary.BigEndian.PutUint32(buf[:4], uint32(len(result)))
		if !writeAll(conn, buf[:4]) {
			return
		}

		// send the encoded (repaired) data
		if !writeAll(conn, result) {
			return
		}

		// early return to avoid sending mutation or crossover results (i.e. non-repaired bytes)
		return
	}

	if wanted&CrossoverBit > 0 || wanted&MutateBit > 0 {
		// send how many bytes of mutated/crossover data will be sent
		binary.BigEndian.PutUint32(buf[:4], uint32(len(result)))
		if !writeAll(conn, buf[:4]) {
			return
		}

		// send the mutated/crossover data
		if !writeAll(conn, result) {
			return
		}
	}
}

func InitServerProcess(pidFile, socketFile string) {
	// isolate the process
	syscall.Setsid()

	// if the file exists the other server process may hang
	if _, err := os.Stat(pidFile); err == nil {

		// check whether it is healthy first
		buf := make([]byte, 1)
		conn, err := net.Dial("unix", socketFile)
		if err == nil {
			conn.SetDeadline(time.Now().Add(10 * time.Millisecond))
			if writeAll(conn, []byte{AreYouAlive}) {
				if readAll(conn, buf) && buf[0] == YesIAmAlive {
					conn.Close()
					os.Exit(0)
				}
			}
		}

		// apparently, the other server process is not healthy, kill it
		data, err := ioutil.ReadFile(pidFile)
		if err != nil {
			panic(err)
		}
		pid, err := strconv.Atoi(string(data))
		if err != nil {
			panic(err)
		}

		// TODO: We should at least check whether the process name is correct and if it exists before
		// killing the PID. However, paper deadline approaches and the risk of killing another process
		// is already low, because PIDs increment and killing a critical process is not possible, since
		// we do not execute as root. Thus, only user processes are in danger if the machine spawns a
		// considerably high amount of processes (PID counter reset), e.g., through a fuzzing job or a reboot.
		// C'est la vie.
		syscall.Kill(pid, syscall.SIGKILL)
	} else if !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}

	// create the pid file
	os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func SendRequest(socketFile string, timeout int, data1, data2 []byte, wanted byte, seedCrossover, seedMutation uint64,
	encoded, decoded *[]byte) bool {
	buf := make([]byte, 8)

	conn, err := net.Dial("unix", socketFile)
	if err != nil {
		return false
	}
	defer conn.Close()

	// set deadline for all connections up to the point just before the encoded data is sent
	conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Millisecond))

	// ask whether the server is alive
	if !writeAll(conn, []byte{AreYouAlive}) {
		return false
	}

	// see whether the server replies as expected
	if ok := readAll(conn, buf[:1]); !ok || buf[0] != YesIAmAlive {
		return false
	}

	// tell the server what we want and how large data1 is
	buf[0] = wanted
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(data1)))
	if !writeAll(conn, buf[:5]) {
		return false
	}

	// send the encoded data
	if !writeAll(conn, data1) {
		// since the handshake was successful, we likely reached a timeout
		// do not try again from now on and discard this attempt
		return true
	}

	if wanted&CrossoverBit > 0 {
		binary.BigEndian.PutUint32(buf[:4], uint32(len(data2)))
		if !writeAll(conn, buf[:4]) {
			return false
		}
		if !writeAll(conn, data2) {
			return true
		}
		binary.BigEndian.PutUint64(buf[:8], seedCrossover)
		if !writeAll(conn, buf[:8]) {
			return true
		}
	}

	if wanted&MutateBit > 0 {
		binary.BigEndian.PutUint64(buf[:8], seedMutation)
		if !writeAll(conn, buf[:8]) {
			return true
		}
	}

	if wanted&DecodeBit > 0 {
		if !readAll(conn, buf[:4]) {
			return true
		}
		nBytes := int(binary.BigEndian.Uint32(buf[:4]))
		*decoded = make([]byte, nBytes)
		// obtain the decoded data
		if !readAll(conn, *decoded) {
			return true
		}
	}

	if wanted&CrossoverBit > 0 || wanted&MutateBit > 0 || wanted&EncodeBit > 0 {
		// obtain the length of the mutated/crossover or encoded data
		if !readAll(conn, buf[:4]) {
			return true
		}
		nBytes := int(binary.BigEndian.Uint32(buf[:4]))
		*encoded = make([]byte, nBytes)

		// receive the encoded data
		if !readAll(conn, *encoded) {
			return true
		}
	}
	return true
}

func RestartServer(lockFile, serverBin string) {
	var file *os.File
	if _, err := os.Stat(lockFile); errors.Is(err, os.ErrNotExist) {
		// apparently, the lock file does not exist, create one and lock it if possible
		file, err = os.Create(lockFile)
		if err != nil {
			return
		}
		defer file.Close()
	} else if err != nil {
		return
	} else {
		file, err = os.OpenFile(lockFile, os.O_RDWR, 0644)
		if err != nil {
			return
		}
		defer file.Close()
	}

	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		// got the lock! now start the server
		exec.Command(serverBin).Start()
	}
}
