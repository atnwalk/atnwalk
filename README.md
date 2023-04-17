# ATNWalk

## Building 

Put your grammars into the `grammars/` folder, e.g.:
```bash
mkdir -p grammars
vim grammars/MyGrammar.g4
```

Build binaries for all grammars in the `grammars/` folder with:
```bash
./build.bash
```

Output:
```
[ all ] Deleting old build/ directory if necessary
[ all ] Creating build/ directory
[ all ] Checking if ANTLR4 variable is set
[ all ] Downloading antlr JAR if necessary
[ all ] Executing: go mod vendor
[ all ] Ensuring that antlr_extension.go is installed
[ all ] Finding targets to compile
[ all ] Targets found: JavaScript Lua Php Ruby SQLite
[...]
[ SQLite ] Generating Go files with ANTLR
[ SQLite ] Generating Go files for atnwalk
[ SQLite ] Running go build commands
```

Find the output files in the `build/` directory, e.g.:
```
build/
├── .antlr-4.11.1-complete.jar
├── [...]
└── sqlite
    ├── bin
    │   ├── client
    │   ├── decode
    │   ├── encode
    │   ├── mutate
    │   └── server
    └── gen
        ├── cmd
        │   ├── client
        │   │   └── main.go
        │   ├── decode
        │   │   └── main.go
        │   ├── encode
        │   │   └── main.go
        │   ├── mutate
        │   │   └── main.go
        │   └── server
        │       └── main.go
        ├── SQLite.interp
        ├── sqlite_lexer.go
        ├── SQLiteLexer.interp
        ├── SQLiteLexer.tokens
        ├── sqlite_parser.go
        └── SQLite.tokens
```

## How to use
CLI Examples (`decode`, `encode`, `mutate`):
```bash
cd ./build/sqlite/bin/

# decoding random 8 bytes for initial input generation
head -c8 /dev/urandom | ./decode

# saving an encoded file to 'encoded.bytes' (STDERR) while still writing the decoded output (STDOUT)
head -c8 /dev/urandom | ./decode -wb 2> encoded.bytes

# verifying that the encoded bytes decode to the same output than previously observed
cat encoded.bytes | ./decode

# mutating the encoded bytes and decode to a new output (STDOUT) 
# writing the encoded bytes into 'new_encoded.bytes' (STDERR)
cat encoded.bytes | ./mutate | ./decode -wb 2> new_encoded.bytes

# performing a crossover (with mutation after crossover) of 'encoded.bytes' and 'new_encoded.bytes', 
# decoding it into a new input (STDOUT), and saving it back to 'crossover.bytes' (STDERR)
./mutate -c encoded.bytes new_encoded.bytes | ./decode -wb 2> crossover.bytes

# decode crossover.bytes
cat crossover.bytes | ./decode | tee crossover.txt

# encode a text to bytes again (slow! don't use this in fuzzing campaigns or other evolutionary algorithms)
cat crossover.txt | ./encode > crossover2.bytes

# make sure that both decoded texts are the same (encoded files may differ)
diff -s <(cat crossover.bytes | ./decode) <(cat crossover2.bytes | ./decode)
```

IPC Examples (`server`, `client`):
```bash
cd ./build/sqlite/bin/

# start the server (in background and not bound to the shell)
nohup ./server &

# use the client to make request to the opened 'atnwalk.socket'
# client must always be executed in the same folder where the 'atnwalk.socket' is

# decode
head -c8 /dev/urandom | ./client -d

# decode (STDOUT) and encode (STDERR)
head -c8 /dev/urandom | ./client -d -e 2> encoded.bytes

# mutate with seed (1234 and 5678) and return encoded bytes (STDERR), no decoding
cat encoded.bytes | ./client -m 1234 -e 2> a.bytes
cat encoded.bytes | ./client -m 5678 -e 2> b.bytes

# crossover with seed (9012 no additional mutation) and return encoded bytes (STDERR), with decoding (STDOUT)
./client -c a.bytes b.bytes 9012 -d -e 2> c1.bytes

# crossover with seed (3333, mutation after crossover seed 5555) and return encoded bytes (STDERR), with decoding (STDOUT)
./client -c a.bytes b.bytes 3333 -m 5555 -d -e 2> c2.bytes

# show previous crossover results (decode only, no encoding, no mutation, no crossover)
cat c1.bytes | ./client -d
cat c2.bytes | ./client -d

# kill the server
kill "$(cat atnwalk.pid)"
```

## Hints
- Use whitespaces in your grammar. The grammar you write is used for generation not for parsing, 
so whitespaces are important.
- Don't use `EOF` in your grammar, it may lead to weird outputs since `EOF` if kindly ignored by atnwalk.
- Avoid using regular expressions in more high-level grammars and use some meaningful samples. For example:
  - To fuzz an int array use some interesting or boundary values like `0`, `1`, `4096`, ...
  - For fuzzing function names use names of built-in functions
