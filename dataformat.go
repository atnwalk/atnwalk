package atnwalk

import (
	"math/bits"
	"math/rand"
	"sort"
)

type Encoder struct {
	data     *[]byte
	position int
	cursor   int
}

func NewEncoder(data []byte) *Encoder {
	if data == nil {
		data = []byte{}
	}
	return &Encoder{data: &data}
}

const ParitySum = 0xdd

func (encoder *Encoder) WriteRuleHeader(ruleIndex int, numRules int, isLexerRule bool) {

	// handle edge cases
	if ruleIndex < 0 {
		panic("the rule index must be greater than or equal to 0")
	} else if ruleIndex >= numRules {
		panic("the rule index must be strictly smaller than the number of rules")
	} else if numRules < 1 {
		panic("the number of rules must be greater than or equal to 1")
	}

	// pad to the next full byte
	if encoder.cursor != 0 {
		encoder.position++
		encoder.cursor = 0
	}

	// calculate the number of bits to store the ruleIndex
	// and the number of bytes that are required to store the header
	// (header = 1 parity byte + full bytes required for requiredBitsRuleIndex)
	requiredBitsRuleIndex := 32 - bits.LeadingZeros32(uint32(numRules-1))
	requiredBytesHeader := 1 + ((requiredBitsRuleIndex + 1) >> 3)
	if requiredBitsRuleIndex%8 > 0 {
		requiredBytesHeader++
	}

	// grow the underlying buffer if necessary
	if (encoder.position + requiredBytesHeader) >= cap(*encoder.data) {
		newBuffer := make([]byte, (len(*encoder.data)+requiredBytesHeader)<<1)
		copy(newBuffer, *encoder.data)
		*encoder.data = newBuffer
	}

	// the rule byte is lexer/parser bit + 7 leading bits of the rule index
	// little endian! thus reversed order
	reversedRuleIndex := bits.Reverse32(uint32(ruleIndex))
	ruleByte := reversedRuleIndex >> 25

	// if it is a parser rule, the leading bit is 1 otherwise 0
	if !isLexerRule {
		ruleByte |= 0x80
	}

	// calculate the parity byte
	parityByte := ParitySum ^ ruleByte

	// write the first two header bytes
	(*encoder.data)[encoder.position] = byte(parityByte)
	(*encoder.data)[encoder.position+1] = byte(ruleByte)
	encoder.position += 2

	// write the remaining bits of the rule index if required
	if requiredBitsRuleIndex > 7 {
		reversedRuleIndex <<= 7
		for i := 0; i <= requiredBitsRuleIndex-8; i += 8 {
			(*encoder.data)[encoder.position] = byte(reversedRuleIndex >> (24 - i))
			encoder.position++
		}
		encoder.cursor = (requiredBitsRuleIndex + 1) % 8
		if encoder.cursor != 0 {
			encoder.position--
		}
	}
}

func (encoder *Encoder) Encode(number, boundary int) {
	if number >= boundary {
		panic("the number must be strictly smaller than the boundary")
	} else if boundary < 1 {
		panic("boundary must be greater than or equal to 1")
	} else if boundary == 1 {
		return
	}

	requiredBits := 32 - bits.LeadingZeros32(uint32(boundary-1))

	if encoder.position >= cap(*encoder.data) {
		newBuffer := make([]byte, (len(*encoder.data)+1)<<1)
		copy(newBuffer, *encoder.data)
		*encoder.data = newBuffer
	}

	var availableBits int
	for requiredBits > 0 {
		availableBits = 8 - encoder.cursor
		if availableBits == 0 {
			encoder.position++
			encoder.cursor = 0
			if encoder.position >= len(*encoder.data) {
				newBuffer := make([]byte, (len(*encoder.data)+1)<<1)
				copy(newBuffer, *encoder.data)
				*encoder.data = newBuffer
			}
			continue
		}
		(*encoder.data)[encoder.position] |= byte((number << (32 - requiredBits)) >> (24 + encoder.cursor))
		if requiredBits > availableBits {
			encoder.cursor += availableBits
			requiredBits -= availableBits
		} else {
			encoder.cursor += requiredBits
			requiredBits = 0
		}
	}
}

func (encoder *Encoder) Bytes() []byte {
	if encoder.cursor != 0 {
		return (*encoder.data)[:encoder.position+1]
	} else {
		return (*encoder.data)[:encoder.position]
	}
}

type headInfo struct {
	isSet       bool
	ruleIndex   int
	numRules    int
	isLexerRule bool
}

type Decoder struct {
	data             []byte
	position         int
	cursor           int
	usePRNG          bool
	prngData         []byte
	prngPosition     int
	prngCursor       int
	prngSource       rand.Source
	parserRules      map[int][]int
	parserRuleBits   int
	lexerRules       map[int][]int
	lexerRuleBits    int
	writeBackEncoder *Encoder
	writeBackHead    headInfo
}

func getRuleNumber(requiredBits int, data []byte) int {
	var ruleNumber uint32
	ruleNumber |= uint32(data[0]) << 25
	remainingBits := requiredBits - 7
	i := 1
	for remainingBits > 0 {
		bitsToRead := 8
		if remainingBits < 8 {
			bitsToRead = remainingBits
		}
		ruleNumber |= uint32(data[i]) >> (8 - bitsToRead) << (32 - (requiredBits - remainingBits) - bitsToRead)
		remainingBits -= bitsToRead
		i++
	}
	return int(bits.Reverse32(ruleNumber))
}

func NewDecoder(data []byte, numParserRules, numLexerRules int, writeBack *[]byte) *Decoder {
	var encoder *Encoder
	if writeBack != nil {
		encoder = &Encoder{data: writeBack}
	}

	if len(data) == 0 {
		return &Decoder{usePRNG: true, writeBackEncoder: encoder, prngSource: rand.NewSource(1)}
	}

	decoder := &Decoder{data: data, lexerRules: map[int][]int{}, parserRules: map[int][]int{}, writeBackEncoder: encoder}
	if numParserRules < 1 || numLexerRules < 1 {
		decoder.prngSource = rand.NewSource(1)
		return decoder
	}
	decoder.parserRuleBits = 32 - bits.LeadingZeros32(uint32(numParserRules-1))
	decoder.lexerRuleBits = 32 - bits.LeadingZeros32(uint32(numLexerRules-1))

	// detect rule headers and set the seed with parity and rule bytes
	seed := int64(data[0])
	for i := 0; i < len(data)-2; i++ {
		// structure: <parity byte> <rule byte> <remaining rule bits>
		if data[i]^data[i+1] == ParitySum {
			// check if it is a parser rule, first bit should be 1
			if data[i+1]&0x80 > 0 {
				ruleNum := getRuleNumber(decoder.parserRuleBits, data[i+1:])
				decoder.parserRules[ruleNum] = append(decoder.parserRules[ruleNum], i)
				// otherwise, it is a lexer
			} else {
				ruleNum := getRuleNumber(decoder.lexerRuleBits, data[i+1:])
				decoder.lexerRules[ruleNum] = append(decoder.lexerRules[ruleNum], i)
			}
			// use parity and rule bytes to init the seed
			seed ^= int64(data[i]) << (56 - (i % 57))
			seed ^= int64(data[i+1]) << ((i + 1) % 57)
		}
	}
	decoder.prngSource = rand.NewSource(seed)

	return decoder
}

func (decoder *Decoder) appendPRNGBytes() {
	nextBytes := decoder.prngSource.Int63()
	decoder.prngData = append(decoder.prngData,
		// Ignore for the moment the first byte as it "misses one bit" at the beginning
		// byte(nextBytes>>56),
		byte(nextBytes&0x00ff000000000000>>48),
		byte(nextBytes&0x0000ff0000000000>>40),
		byte(nextBytes&0x000000ff00000000>>32),
		byte(nextBytes&0x00000000ff000000>>24),
		byte(nextBytes&0x0000000000ff0000>>16),
		byte(nextBytes&0x000000000000ff00>>8),
		byte(nextBytes&0x00000000000000ff))
}

func (decoder *Decoder) Init(ruleIndex int, isLexerRule bool) {
	decoder.writeBackHead.isSet = false
	decoder.usePRNG = false

	if decoder.writeBackEncoder != nil {
		decoder.writeBackHead.ruleIndex = ruleIndex
		if !isLexerRule {
			decoder.writeBackHead.numRules = 0x1 << decoder.parserRuleBits
		} else {
			decoder.writeBackHead.numRules = 0x1 << decoder.lexerRuleBits
		}
		decoder.writeBackHead.isLexerRule = isLexerRule
	}

	var decoderRules map[int][]int
	var bitsToAdvance int
	if !isLexerRule {
		decoderRules = decoder.parserRules
		// parity byte + one bit whether lexer or parser = 9
		bitsToAdvance = 9 + decoder.parserRuleBits
	} else {
		decoderRules = decoder.lexerRules
		// parity byte + one bit whether lexer or parser = 9
		bitsToAdvance = 9 + decoder.lexerRuleBits
	}
	if bitsToAdvance < 16 {
		bitsToAdvance = 16
	}

	if positions, ok := decoderRules[ruleIndex]; ok {
		if len(positions) == 0 {
			delete(decoderRules, ruleIndex)
			decoder.usePRNG = true
			return
		}
		index := sort.Search(len(positions), func(i int) bool { return positions[i] >= decoder.position })
		if index >= len(positions) {
			index = 0
		}
		decoder.position = positions[index] + (bitsToAdvance >> 3)
		decoder.cursor = bitsToAdvance % 8
		copy(positions[index:], positions[index+1:])
		decoderRules[ruleIndex] = positions[:len(positions)-1]
	} else {
		decoder.usePRNG = true
	}

}

func (decoder *Decoder) Decode(boundary int) int {
	if boundary < 1 {
		panic("boundary must be greater than or equal to 1")
	}

	if boundary == 1 {
		return 0
	}

	var data []byte
	var position, cursor int
	if decoder.data == nil || decoder.usePRNG {
		if decoder.prngData == nil || decoder.prngPosition >= len(decoder.prngData) {
			decoder.appendPRNGBytes()
		}
		decoder.usePRNG = true
		data = decoder.prngData
		position = decoder.prngPosition
		cursor = decoder.prngCursor
	} else {
		data = decoder.data
		position = decoder.position
		cursor = decoder.cursor
	}

	requiredBits := 32 - bits.LeadingZeros32(uint32(boundary-1))

	if position >= len(data) {
		if !decoder.usePRNG {
			// save the position and cursor of the original data array
			decoder.position = position
			decoder.cursor = cursor
			// switch to use prng
			decoder.usePRNG = true
			position = decoder.prngPosition
			cursor = decoder.prngCursor
		}
		if decoder.prngData == nil || position >= len(decoder.prngData) {
			decoder.appendPRNGBytes()
		}
		data = decoder.prngData
	}

	var result uint32
	var availableBits, numBitsToRead int
	for requiredBits > 0 {
		availableBits = 8 - cursor
		if availableBits == 0 {
			position++
			cursor = 0
			if position >= len(data) {
				if !decoder.usePRNG {
					// save the position and cursor of the original data array
					decoder.position = position
					decoder.cursor = cursor
					// switch to use PRNGs position and cursor onwards
					position = decoder.prngPosition
					cursor = decoder.prngCursor
				}
				decoder.usePRNG = true
				data = decoder.prngData
				// append prng bytes if required
				if decoder.prngData == nil || position >= len(decoder.prngData) {
					decoder.appendPRNGBytes()
					data = decoder.prngData
				}
			}
			continue
		}
		// make space for 'numBitsToRead' amount of bits in the 'result' integer by left shift,
		// thus, saving the bits that were already read from the 'data' byte
		if numBitsToRead = requiredBits; availableBits < requiredBits {
			numBitsToRead = availableBits
		}
		result <<= numBitsToRead

		// if reading the 'data' byte only partially then remove the leading bits until the cursor by overflowing,
		// shift the required bits back into position to assign them to the 'result' integer
		// otherwise no  shift is required
		if cursor != 0 || numBitsToRead != 8 {
			result |= (uint32(data[position]) << (cursor + 24)) >> (32 - numBitsToRead)
		} else {
			result |= uint32(data[position])
		}
		requiredBits -= numBitsToRead
		cursor += numBitsToRead
	}

	// fix the last cursor position to know before calling this function
	// whether to use the prng or not
	if cursor == 8 {
		cursor = 0
		position++
	}

	// save the correct position and cursor
	if !decoder.usePRNG {
		decoder.position = position
		decoder.cursor = cursor
		if decoder.position >= len(decoder.data) {
			decoder.usePRNG = true
		}
	} else {
		decoder.prngPosition = position
		decoder.prngCursor = cursor
	}

	if decoder.writeBackEncoder != nil {
		if !decoder.writeBackHead.isSet {
			decoder.writeBackEncoder.WriteRuleHeader(decoder.writeBackHead.ruleIndex, decoder.writeBackHead.numRules, decoder.writeBackHead.isLexerRule)
			decoder.writeBackHead.isSet = true
		}
		decoder.writeBackEncoder.Encode(int(result%uint32(boundary)), boundary)
	}

	return int(result % uint32(boundary))
}
