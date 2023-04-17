package atnwalk

import (
	"math/bits"
	"math/rand"
)

type PRNG struct {
	source    rand.Source
	number    uint64
	available int
}

func NewPRNG(seed int64) *PRNG {
	return &PRNG{source: rand.NewSource(seed)}
}

func (prng *PRNG) Int(boundary int) int {
	if boundary < 1 {
		panic("boundary must be greater than or equal to 1")
	} else if boundary == 1 {
		return 0
	}
	maxNumber := uint64(boundary - 1)
	requiredBits := bits.Len64(maxNumber)
	mask := uint64(0xffffffffffffffff >> (64 - requiredBits))

	var x uint64
	for {
		if prng.available < requiredBits {
			newNumber := uint64(prng.source.Int63())
			x = (prng.number | (newNumber << prng.available)) & mask
			prng.number = newNumber >> (requiredBits - prng.available)
			prng.available = 63 - requiredBits + prng.available
		} else {
			x = prng.number & mask
			prng.available -= requiredBits
			prng.number >>= requiredBits
		}
		if x <= maxNumber {
			return int(x)
		}
	}
}

func Mutate(data []byte, seed int64) []byte {
	// handle empty data
	if len(data) == 0 {
		return []byte{}
	}

	// init the PRNG
	prng := NewPRNG(seed)

	// mdata holds the mutated data and is initialized with twice the size of the original data
	mdata := make([]byte, len(data), len(data)<<1)
	copy(mdata, data)

	// perform up to 8 different mutations
	for i := 0; i <= prng.Int(8); i++ {

		// select the mutation operation
		switch prng.Int(4) {

		// set byte to random value, avoid no-ops with XOR
		case 0:
			j := prng.Int(len(mdata))
			mdata[j] ^= byte(prng.Int(256))

		// perform addition or subtraction with a random Number in [1..16] on a byte (little endian)
		case 1:
			// we perform little endian operations because small arithmetic operations will mostly target padded bits
			// which would result in a no-op when mutated
			j := prng.Int(len(mdata))
			x := uint8(prng.Int(16) + 1)
			if prng.Int(2) == 0 {
				mdata[j] = bits.Reverse8(bits.Reverse8(mdata[j]) + x)
			} else {
				mdata[j] = bits.Reverse8(bits.Reverse8(mdata[j]) - x)
			}

		// bit flips, either 1/8, 2/8, 4/8, or 8/8 neighboured bits are flipped at a random bit-position in the byte
		case 2:
			numBits := 1 << prng.Int(4)
			j := prng.Int(len(mdata))
			mdata[j] = mdata[j] ^ (byte((1<<numBits)-1) << prng.Int(9-numBits))

		// clone one or multiple existing bytes at a random location (either insert or overwrite)
		case 3:
			// select which mdata[a:b] bytes to clone to position j
			j := prng.Int(len(mdata) + 1)
			a := prng.Int(len(mdata))
			b := a + prng.Int(len(mdata)-a) + 1
			clone := mdata[a:b]

			// shall we insert or overwrite?
			if prng.Int(2) == 0 {
				// insert
				if cap(mdata) < (len(mdata) + len(clone)) {
					// create a new buffer that has twice the required capacity and copy the data
					newBuffer := make([]byte, len(mdata)+len(clone), (len(mdata)+len(clone))<<1)
					copy(newBuffer[:j], mdata[:j])
					copy(newBuffer[j:j+b-a], clone)
					copy(newBuffer[j+b-a:], mdata[j:])
					mdata = newBuffer
				} else {
					// resize but not reallocate the slice and copy the last elements up to the max insertion point
					mdata = mdata[:len(mdata)+b-a]
					for k := len(mdata) - 1; k >= (j + b - a); k-- {
						mdata[k] = mdata[k-b+a]
					}
					// copy data from the clone which is now located either before the insertion area or after
					for k := 0; k < (b - a); k++ {
						if (a + k) < j {
							mdata[j+k] = mdata[a+k]
						} else {
							mdata[j+k] = mdata[b+k]
						}
					}
				}
			} else {
				// overwrite
				if cap(mdata) < j+len(clone) {
					// create a new buffer that has twice the required capacity and copy the data
					newBuffer := make([]byte, j+len(clone), (j+len(clone))<<1)
					copy(newBuffer[:j], mdata[:j])
					copy(newBuffer[j:j+b-a], clone)
					if len(mdata) > j+len(clone) {
						copy(newBuffer[j+b-a:], mdata[j+b-a:])
					}
					mdata = newBuffer
				} else {
					// resize but not reallocate the slice
					if len(mdata) < j+len(clone) {
						mdata = mdata[:j+len(clone)]
					}
					if a > j || b <= j {
						copy(mdata[j:], mdata[a:b])
					} else {
						for k := b - a - 1; k >= 0; k-- {
							mdata[j+k] = mdata[a+k]
						}
					}
				}
			}
		}
	}
	return mdata
}

func Crossover(data1, data2 []byte, seed int64) []byte {
	// handle empty data
	if len(data1) == 0 || len(data2) == 0 {
		cdata := make([]byte, len(data1)+len(data2))
		copy(cdata, data1)
		copy(cdata[len(data1):], data2)
		return cdata
	}

	// init the PRNG
	prng := NewPRNG(seed)

	// single or double crossover?
	if (len(data1) < 2 || len(data2) < 2) || prng.Int(2) == 0 {
		// single cross over
		// split data1 at a and data2 at b, then concatenate data1[:a] with data2[b:]
		a := prng.Int(len(data1)) + 1
		b := prng.Int(len(data2))
		cdata := make([]byte, a+len(data2)-b)
		copy(cdata, data1[:a])
		copy(cdata[a:], data2[b:])
		return cdata
	} else {
		// double cross over
		// split data1 at a1 and a2 and data2 at b1 and b2, then concatenate data1[:a1], data2[b1:b2], and data1[a2:]
		a1 := prng.Int(len(data1)-1) + 1
		a2 := a1 + prng.Int(len(data1)-a1)
		b1 := prng.Int(len(data2))
		b2 := b1 + prng.Int(len(data2)-b1) + 1
		cdata := make([]byte, a1+b2-b1+len(data1)-a2)
		copy(cdata[:a1], data1[:a1])
		copy(cdata[a1:a1+b2-b1], data2[b1:b2])
		copy(cdata[a1+b2-b1:], data1[a2:])
		return cdata
	}
}
