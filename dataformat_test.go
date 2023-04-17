package atnwalk

import (
	"math"
	"testing"
)

func TestDecoder_Decode(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		args []int
		want []uint
	}{
		{
			"Obtain valid next int from the underlying data array",
			[]byte{
				0b00010111, 0b11010110, 0b11010110, 0b11011011, 0b01000111, 0b11000111,
				0b01110101, 0b11000011, 0b11111011},
			[]int{12, 17, 18, 2, 5, 3, 5000, math.MaxInt32, 1, 16},
			[]uint{
				0b0001 % 12, 0b01111 % 17, 0b10101 % 18, 0b1 % 2, 0b011 % 5, 0b01 % 3,
				0b0110110110110 % 5000, 0b1000111110001110111010111000011 % math.MaxInt32,
				0, 0b1111 % 16}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewDecoder(tt.data, 128, 255, nil)
			for i, boundary := range tt.args {
				if got := decoder.Decode(boundary); got != int(tt.want[i]) {
					t.Errorf("decoder.Decode(%v) = %v, want %v", boundary, got, tt.want[i])
				}
			}
		})
	}
}

func TestEncoder_WriteRuleHeader(t *testing.T) {

	type args struct {
		ruleIndex   int
		numRules    int
		isLexerRule bool
	}
	type want struct {
		data     []byte
		position int
		cursor   int
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			"Parser rule 172 (8-bit)",
			args{0b10101100, 255, false},
			want{[]byte{0b01000111, 0b10011010, 0b10000000}, 2, 1}},
		{
			"Lexer rule 172 (8-bit)",
			args{0b10101100, 255, true},
			want{[]byte{0b11000111, 0b00011010, 0b10000000}, 2, 1}},
		{
			"Parser rule 165 (8-bit)",
			args{0b10100101, 255, false},
			want{[]byte{0b00001111, 0b11010010, 0b10000000}, 2, 1}},
		{
			"Lexer rule 165 (8-bit)",
			args{0b10100101, 255, true},
			want{[]byte{0b10001111, 0b01010010, 0b10000000}, 2, 1}},
		{
			"Parser rule 44 (7-bit)",
			args{0b00101100, 99, false},
			want{[]byte{0b01000111, 0b10011010}, 2, 0}},
		{
			"Lexer rule 44 (7-bit)",
			args{0b00101100, 99, true},
			want{[]byte{0b11000111, 0b00011010}, 2, 0}},
		{
			"Parser rule 12 (4-bit)",
			args{0b00001100, 60, false},
			want{[]byte{0b01000101, 0b10011000}, 2, 0}},
		{
			"Lexer rule 12 (4-bit)",
			args{0b00001100, 60, true},
			want{[]byte{0b11000101, 0b00011000}, 2, 0}},
		{
			"Parser Rule 1,517 (11-bit)",
			args{0b10111101101, 1600, false},
			want{[]byte{0b00000110, 0b11011011, 0b11010000}, 2, 4}},
		{
			"Lexer Rule 14,130 (14-bit)",
			args{0b11011100110010, 14322, true},
			want{[]byte{0b11111011, 0b00100110, 0b01110110}, 2, 7}},
		{
			"Parser Rule 11,466 (16-bit)",
			args{0b0010110011001010, 49630, false},
			want{[]byte{0b01110100, 0b10101001, 0b10011010, 0}, 3, 1}},
		{
			"Lexer Rule 5,871,081 (23-bit)",
			args{0b0010110011001010111101001, 6000000, true},
			want{[]byte{0b10010110, 0b01001011, 0b11010100, 0b11001101}, 4, 0}},
		{
			"Parser Rule MaxInt32-1 (31-bit)",
			args{math.MaxInt32 - 1, math.MaxInt32, false},
			want{[]byte{0b01100010, 0b10111111, 0xff, 0xff, 0xff}, 5, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := NewEncoder(nil)
			encoder.WriteRuleHeader(tt.args.ruleIndex, tt.args.numRules, tt.args.isLexerRule)
			for i, b := range tt.want.data {
				if (*encoder.data)[i] != b {
					t.Errorf("encoder.data[%v] = %v, but want %v", i, (*encoder.data)[i], b)
				}
			}
			if byte(encoder.position) != byte(tt.want.position) {
				t.Errorf("encoder.position = %v, but want %v", encoder.position, tt.want.position)
			}
			if byte(encoder.cursor) != byte(tt.want.cursor) {
				t.Errorf("encoder.cursor = %v, but want %v", encoder.cursor, tt.want.cursor)
			}
		})
	}
}

func TestNewDecoder(t *testing.T) {
	type args struct {
		data           []byte
		numParserRules int
		numLexerRules  int
	}
	type want struct {
		parserRules map[int][]int
		lexerRules  map[int][]int
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			"Parser and lexer rules that should be parsed as expected.",
			args{[]byte{
				// gibberish
				23,
				// parser rule 44 at byte 1
				0b01000111, 0b10011010,
				// gibberish
				5, 23, 100, 234, 255, 0,
				// lexer rule 172 at byte 9
				0b11000111, 0b00011010, 0b10000010,
				// gibberish
				42, 8, 200, 128, 3, 99, 251,
				// parser rule 12 at byte 19
				0b01000101, 0b10011000,
				// lexer rule 172 at byte 21
				0b11000111, 0b00011010, 0b11111111,
				// gibberish
				7}, 128, 256},
			want{map[int][]int{44: {1}, 12: {19}}, map[int][]int{172: {9, 21}}}},
		{
			"Parser and lexer rules but last lexer rule will not be recognized because it does not contain data.",
			args{[]byte{
				// gibberish
				23,
				// parser rule 44 at byte 1
				0b01000111, 0b10011010,
				// gibberish
				5, 23, 100, 234, 255, 0,
				// lexer rule 172 at byte 9
				0b11000111, 0b00011010, 0b10000010,
				// gibberish
				42, 8, 200, 128, 3, 99, 251,
				// parser rule 12 at byte 19
				0b01000101, 0b10011000,
				// lexer rule 172 at byte 21 (should not be detected, cannot contain ATN data)
				0b11000111, 0b00011010}, 128, 256},
			want{map[int][]int{44: {1}, 12: {19}}, map[int][]int{172: {9}}}},
		{
			"Parser and lexer rules first rule starts on first byte and last lexer rule will not be recognized " +
				"because it does not contain data.",
			args{[]byte{
				// parser rule 44 at byte 0 (should not be detected, first byte cannot contain a parity byte)
				0b01000111, 0b10011010,
				// gibberish
				5, 23, 100, 234, 255, 0,
				// lexer rule 172 at byte 8
				0b11000111, 0b00011010, 0b10000010,
				// gibberish
				42, 8, 200, 128, 3, 99, 251,
				// parser rule 12 at byte 18
				0b01000101, 0b10011000,
				// lexer rule 172 at byte 21 (should not be detected, cannot contain ATN data)
				0b11000111, 0b00011010}, 128, 256},
			want{map[int][]int{44: {0}, 12: {18}}, map[int][]int{172: {8}}}},
		{
			"Large parser and lexer rules.",
			args{[]byte{
				// gibberish
				4, 2, 3, 6, 8, 33, 12, 55, 83, 34, 152,
				// lexer rule 14,130 at byte 11
				0b11111011, 0b00100110, 0b01110111,
				// parser rule MaxInt32-1 at byte 14
				0b01100010, 0b10111111, 0xff, 0xff, 0xff,
				65, 3, 34, 4, 5, 7}, math.MaxInt32, 14322},
			want{map[int][]int{math.MaxInt32 - 1: {14}}, map[int][]int{14130: {11}}},
			// "Lexer Rule 5,871,081 (23-bit)",
			// args{0b0010110011001010111101001, 6000000, true},
			// want{[]byte{0b10010110, 0b01001011, 0b11010100, 0b11001101}, 4, 0}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewDecoder(tt.args.data, tt.args.numParserRules, tt.args.numLexerRules, nil)
			if len(decoder.parserRules) != len(tt.want.parserRules) {
				t.Errorf("len(decoder.parserRules) = %v, want %v)", len(decoder.parserRules), len(tt.want.parserRules))
			}
			for rule, locations := range decoder.parserRules {
				if len(locations) != len(tt.want.parserRules[rule]) {
					t.Errorf("len(locations) = %v, want %v", len(locations), len(tt.want.parserRules[rule]))
				}
				for i := 0; i < len(locations); i++ {
					if locations[i] != tt.want.parserRules[rule][i] {
						t.Errorf("locations[i] = %v, want %v", locations[i], tt.want.parserRules[rule][i])
					}
				}
			}
			if len(decoder.lexerRules) != len(tt.want.lexerRules) {
				t.Errorf("len(decoder.lexerRules) = %v, want %v)", len(decoder.lexerRules), len(tt.want.lexerRules))
			}
			for rule, locations := range decoder.lexerRules {
				if len(locations) != len(tt.want.lexerRules[rule]) {
					t.Errorf("len(locations) = %v, want %v", len(locations), len(tt.want.lexerRules[rule]))
				}
				for i := 0; i < len(locations); i++ {
					if locations[i] != tt.want.lexerRules[rule][i] {
						t.Errorf("locations[i] = %v, want %v", locations[i], tt.want.lexerRules[rule][i])
					}
				}
			}
		})
	}
}
