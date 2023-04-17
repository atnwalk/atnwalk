#!/bin/bash

function cleanup_and_create_build_dir() {
  echo "[ all ] Deleting contents of build/* directory if necessary"
  test -d "${SCRIPT_DIR}"/build && rm -rf "${SCRIPT_DIR}"/build/*

  echo "[ all ] Creating build/ directory"
  mkdir -p "${SCRIPT_DIR}"/build
}

function check_antlr4() {
  echo "[ all ] Checking if ANTLR4 variable is set"
  if [[ -z ${ANTLR4+x} ]]; then
        export ANTLR4="${SCRIPT_DIR}/build/.antlr-4.11.1-complete.jar"
  fi
  echo "[ all ] Downloading antlr JAR if necessary"
  test -f "${ANTLR4}" || (cd "$(dirname ${ANTLR4})" && curl --output ${ANTLR4} https://www.antlr.org/download/antlr-4.11.1-complete.jar)}

function run_go_mod_vendor() {
  echo "[ all ] Executing: go mod vendor"
  go mod vendor
}

function install_antlr_extension() {
  echo "[ all ] Ensuring that antlr_extension.go is installed"
  cp _antlr_export.go vendor/github.com/antlr/antlr4/runtime/Go/antlr/v4/antlr_export.go
}

function get_targets() {
  echo "[ all ] Finding targets to compile"
  local -n _targets=$1
  _targets=( $( find ./grammars/ -name '*.g4' -print0 |
    xargs -0 -I'{}' basename '{}' |
    sed -n -E 's/([A-Z][A-Za-z0-9]+)(Lexer|Parser)\.g4/\1/p' |
    sort |
    uniq ) )
  if find ./grammars/ -name '*.g4' -print0 | xargs -0 -I'{}' basename '{}' | grep -vE '^.*Lexer\.g4$' | grep -vE '^.*Parser\.g4$' > /dev/null; then
    _targets+=( $(find ./grammars/ -name '*.g4' -print0 | xargs -0 -I'{}' basename '{}' |
      grep -vE '^.*Lexer\.g4$' |
      grep -vE '^.*Parser\.g4$' |
      sed -n -E 's/([A-Z][A-Za-z0-9]+)\.g4/\1/p' |
      sort |
      uniq ) )
  fi
  echo "[ all ] Targets found: ${_targets[*]}"
}

function generate_antlr_go_files() {
  echo "[ ${1} ] Generating Go files with ANTLR"
  mkdir -p "${SCRIPT_DIR}"/build/"${1,,}"/gen/
  cd grammars/
  java -Xmx500M -cp ".:${ANTLR4}" org.antlr.v4.Tool -Dlanguage=Go \
	  -no-listener -no-visitor -package parser -o "${SCRIPT_DIR}"/build/"${1,,}"/gen/ "${1}"*.g4
  cd - > /dev/null
}

function generate_atnwalk_go_files() {
  echo "[ ${1} ] Generating Go files for atnwalk"
  mkdir -p "${SCRIPT_DIR}"/build/"${1,,}"/{gen,bin}/
  cp -r cmd "${SCRIPT_DIR}"/build/"${1,,}"/gen/

  ###############################################
  # build/<grammar_name>/gen/cmd/decode/main.go #
  ###############################################
  cat <(sed -E '/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ IMPORT[ \t]*$/q' "${SCRIPT_DIR}"/cmd/decode/main.go) <(cat <<EOF
parser "atnwalk/build/${1,,}/gen"
EOF
) <(sed -n -E '/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ IMPORT[ \t]*$/,/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ EXEC[ \t]*$/p' "${SCRIPT_DIR}"/cmd/decode/main.go | sed '1d') <(cat <<EOF
parser_ = parser.New${1}Parser(nil)
lexer = parser.New${1}Lexer(nil)
EOF
  ) <(sed -E '0,/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ EXEC[ \t]*$/d' "${SCRIPT_DIR}"/cmd/decode/main.go) > "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/decode/main.go
  go fmt "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/decode/main.go > /dev/null

  ###############################################
  # build/<grammar_name>/gen/cmd/encode/main.go #
  ###############################################
  cat <(sed -E '/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ IMPORT[ \t]*$/q' "${SCRIPT_DIR}"/cmd/encode/main.go) <(cat <<EOF
parser "atnwalk/build/${1,,}/gen"
"atnwalk"
EOF
  ) <(sed -n -E '/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ IMPORT[ \t]*$/,/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ EXEC[ \t]*$/p' "${SCRIPT_DIR}"/cmd/encode/main.go | sed '1d') <(cat <<EOF
// create the input and token streams, lexer, and parser instances
input := antlr.NewInputStream(string(data))
lexer := parser.New${1}Lexer(input)
stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
parser_ := parser.New${1}Parser(stream)

// call the main rule that shall be parsed
tree := parser_.Start()

// create an ATNWalker
walker := atnwalk.NewATNWalker(parser_, lexer)
os.Stdout.Write(walker.Encode(tree))

// this is just a safety measure to make sure that both, lexer and parser, have been properly initialized
_parser_ = parser_
_lexer = lexer
EOF
    ) <(sed -E '0,/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ EXEC[ \t]*$/d' "${SCRIPT_DIR}"/cmd/encode/main.go) > "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/encode/main.go
    go fmt "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/encode/main.go > /dev/null

  ###############################################
  # build/<grammar_name>/gen/cmd/server/main.go #
  ###############################################
  cat <(sed -E '/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ IMPORT[ \t]*$/q' "${SCRIPT_DIR}"/cmd/server/main.go) <(cat <<EOF
parser "atnwalk/build/${1,,}/gen"
EOF
) <(sed -n -E '/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ IMPORT[ \t]*$/,/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ EXEC[ \t]*$/p' "${SCRIPT_DIR}"/cmd/server/main.go | sed '1d') <(cat <<EOF
parser_ = parser.New${1}Parser(nil)
lexer = parser.New${1}Lexer(nil)
EOF
  ) <(sed -E '0,/^[ \t]*\/\/\ DO\ NOT\ REMOVE\ THIS\ LINE\ \-\ EXEC[ \t]*$/d' "${SCRIPT_DIR}"/cmd/server/main.go) > "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/server/main.go
  go fmt "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/server/main.go > /dev/null
}

function run_go_build() {
  echo "[ ${1} ] Running go build commands"
  go build -o "${SCRIPT_DIR}"/build/"${1,,}"/bin/client "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/client/main.go
  go build -o "${SCRIPT_DIR}"/build/"${1,,}"/bin/decode "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/decode/main.go
  go build -o "${SCRIPT_DIR}"/build/"${1,,}"/bin/encode "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/encode/main.go
  go build -o "${SCRIPT_DIR}"/build/"${1,,}"/bin/mutate "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/mutate/main.go
  go build -o "${SCRIPT_DIR}"/build/"${1,,}"/bin/server "${SCRIPT_DIR}"/build/"${1,,}"/gen/cmd/server/main.go
}

function runtests() {
  for i in {001..10000}
  do
    head -c8 /dev/urandom | tee blob.bytes | ./decode -wb > decoded.txt 2> wb.encoded.bytes
    cat decoded.txt | ../atnencode/encode > orig.encoded.bytes
    printf "\r                    \r"
    printf "\r${i}: "$(ls -l decoded.txt | awk '{print $5}')
    diff <(cat wb.encoded.bytes | ./decode) <(cat orig.encoded.bytes | ./decode)
    if [[ $? -ne 0 ]]; then
      echo "ERROR"
      break;
    fi
    diff <(cat wb.encoded.bytes | ./decode) decoded.txt
    if [[ $? -ne 0 ]]; then
      echo "ERROR"
      break;
    fi
  done
  printf "\n"
}

function main() {
  set -euo pipefail
  export SCRIPT_DIR=$(realpath "$(dirname "${BASH_SOURCE[0]}")")
  cd "${SCRIPT_DIR}"
  cleanup_and_create_build_dir
  check_antlr4
  run_go_mod_vendor
  install_antlr_extension
  local targets
  get_targets targets
  for f in "${targets[@]}"
  do
    generate_antlr_go_files "${f}"
    generate_atnwalk_go_files "${f}"
    run_go_build "${f}"
    # TODO: runtests
  done
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main
fi
