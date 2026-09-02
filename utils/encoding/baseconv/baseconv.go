// Package baseconv encodes and decodes byte slices with a caller-chosen
// alphabet.
//
// Power-of-two alphabets use a bitwise path (a 32-character set interops
// with encoding/base32). Other lengths (base62) use a big-integer path.
// Bitwise decode rejects non-canonical leftover bits (ErrNonCanonical);
// the math path has no equivalent check.
//
// AlphabetBase32 is Crockford's set without I, L, O, or U (32 characters).
// StdRaw32Encoding / StdRaw62Encoding use '=' padding. That is the opposite
// of encoding/base32.Raw*, which means unpadded.
package baseconv

import (
	"errors"
	"fmt"
	"math/bits"
	"slices"
	"strings"
)

// ErrNonCanonical reports input that decodes cleanly but is not what the
// encoder would have produced for the resulting bytes. Accepting such input
// would make decoding many-to-one, so two different strings could stand for the
// same value — which breaks any use of the encoded form as an identifier.
var ErrNonCanonical = errors.New("baseconv: non-canonical encoding")

// BaseEncoding provides customizable base encoding/decoding functionality.
// It supports arbitrary alphabets and optional padding characters for flexible encoding schemes.
type BaseEncoding struct {
	alphabet  string
	base      int
	decodeMap map[byte]int
	padChar   byte
}

// NewBaseEncoding creates a new base encoding instance with the specified alphabet.
// The alphabet defines the character set used for encoding and must contain at least 2 unique characters.
func NewBaseEncoding(alphabet string) (*BaseEncoding, error) {
	return NewBaseEncodingWithPadding(alphabet, 0)
}

// NewBaseEncodingWithPadding creates a new base encoding instance with alphabet and padding character.
// The padding character is used to align encoded output and must not conflict with alphabet characters.
func NewBaseEncodingWithPadding(alphabet string, padChar byte) (*BaseEncoding, error) {
	if len(alphabet) < 2 {
		return nil, errors.New("alphabet must have at least 2 characters")
	}

	decodeMap := make(map[byte]int, len(alphabet))
	for i := 0; i < len(alphabet); i++ {
		char := alphabet[i]
		if _, exists := decodeMap[char]; exists {
			return nil, errors.New("alphabet contains duplicate characters")
		}
		decodeMap[char] = i
	}

	if _, exists := decodeMap[padChar]; padChar != 0 && exists {
		return nil, errors.New("padding character conflicts with alphabet")
	}

	return &BaseEncoding{
		alphabet:  alphabet,
		base:      len(alphabet),
		decodeMap: decodeMap,
		padChar:   padChar,
	}, nil
}

// EncodeToString encodes binary data to a string using the configured base encoding.
// It automatically selects the most efficient encoding method based on the alphabet size.
func (e *BaseEncoding) EncodeToString(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	bitsPerChar, bitwise := powerOfTwoBits(e.base)
	if !bitwise {
		return e.encodeMathematical(data)
	}

	return e.encodeBitwise(data, bitsPerChar)
}

func (e *BaseEncoding) encodeBitwise(data []byte, bitsPerChar int) string {
	var result strings.Builder
	var buffer uint32
	var bitsInBuffer int

	for _, b := range data {
		buffer = (buffer << 8) | uint32(b)
		bitsInBuffer += 8

		for bitsInBuffer >= bitsPerChar {
			bitsInBuffer -= bitsPerChar
			index := (buffer >> bitsInBuffer) & ((1 << bitsPerChar) - 1)
			result.WriteByte(e.alphabet[index])
		}
	}

	if bitsInBuffer > 0 {
		buffer <<= bitsPerChar - bitsInBuffer
		index := buffer & ((1 << bitsPerChar) - 1)
		result.WriteByte(e.alphabet[index])
	}

	if e.padChar != 0 {
		inputBits := len(data) * 8
		outputChars := (inputBits + bitsPerChar - 1) / bitsPerChar
		paddingNeeded := 0

		switch bitsPerChar {
		case 6: // base64
			paddingNeeded = (4 - (outputChars % 4)) % 4
		case 5: // base32
			paddingNeeded = (8 - (outputChars % 8)) % 8
		}

		for i := 0; i < paddingNeeded; i++ {
			result.WriteByte(e.padChar)
		}
	}

	return result.String()
}

func (e *BaseEncoding) encodeMathematical(data []byte) string {
	leadingZeros := 0
	for _, b := range data {
		if b == 0 {
			leadingZeros++
		} else {
			break
		}
	}

	digits := make([]int, 0)
	temp := make([]int, len(data)-leadingZeros)
	for i, b := range data[leadingZeros:] {
		temp[i] = int(b)
	}

	for len(temp) > 0 {
		carry := 0
		newTemp := make([]int, 0)

		for _, digit := range temp {
			carry = carry*256 + digit
			if carry >= e.base || len(newTemp) > 0 {
				newTemp = append(newTemp, carry/e.base)
			}
			carry = carry % e.base
		}

		digits = append(digits, carry)
		temp = newTemp
	}
	slices.Reverse(digits)

	var result strings.Builder
	for i := 0; i < leadingZeros; i++ {
		result.WriteByte(e.alphabet[0])
	}
	for _, digit := range digits {
		result.WriteByte(e.alphabet[digit])
	}

	return result.String()
}

// DecodeString decodes a base-encoded string back to binary data.
// It automatically handles padding removal and selects the appropriate decoding method
// based on the alphabet size. Returns an error if the input contains invalid characters.
func (e *BaseEncoding) DecodeString(encoded string) ([]byte, error) {
	if len(encoded) == 0 {
		return []byte{}, nil
	}

	data := encoded
	if e.padChar != 0 {
		data = strings.TrimRight(encoded, string(e.padChar))
	}

	if len(data) == 0 {
		return []byte{}, nil
	}

	for i := 0; i < len(data); i++ {
		if _, exists := e.decodeMap[data[i]]; !exists {
			return nil, fmt.Errorf("invalid character '%c' at position %d", data[i], i)
		}
	}

	bitsPerChar, bitwise := powerOfTwoBits(e.base)
	if !bitwise {
		return e.decodeMathematical(data)
	}

	return e.decodeBitwise(data, bitsPerChar)
}

func powerOfTwoBits(base int) (int, bool) {
	if base&(base-1) != 0 {
		return 0, false
	}
	return bits.Len(uint(base)) - 1, true
}

func (e *BaseEncoding) decodeBitwise(data string, bitsPerChar int) ([]byte, error) {
	var result []byte
	var buffer uint32
	var bitsInBuffer int

	for i := 0; i < len(data); i++ {
		value := e.decodeMap[data[i]]
		buffer = (buffer << bitsPerChar) | uint32(value)
		bitsInBuffer += bitsPerChar

		for bitsInBuffer >= 8 {
			bitsInBuffer -= 8
			b := byte((buffer >> bitsInBuffer) & 0xFF)
			result = append(result, b)
		}
	}

	// Reject anything the encoder could not have produced, so a value has exactly
	// one valid encoding. Both checks below used to pass silently, which let
	// distinct strings decode to identical bytes — ruinous wherever an encoded
	// string is used as an identifier, because two of them then denote the same
	// entity and slip past any deduplication done on the string.
	//
	// A trailing partial group is padding and must be zero: with 5 bits per
	// character the last character of a 13-character base32 string contributes
	// one bit beyond the eighth byte, so "…2" and "…3" differed only in a bit
	// that was discarded.
	if bitsInBuffer > 0 && buffer&((1<<bitsInBuffer)-1) != 0 {
		return nil, fmt.Errorf("%w: non-zero padding bits", ErrNonCanonical)
	}
	// And the partial group must be shorter than one character, otherwise the
	// input carries a character that encodes nothing: 14 base32 characters hold
	// six leftover bits, one more than a character's worth, so the fourteenth
	// could be dropped without changing the result.
	if bitsInBuffer >= bitsPerChar {
		return nil, fmt.Errorf("%w: input has %d trailing bits, at most %d expected", ErrNonCanonical, bitsInBuffer, bitsPerChar-1)
	}

	return result, nil
}

func (e *BaseEncoding) decodeMathematical(data string) ([]byte, error) {
	leadingZeros := 0
	firstChar := e.alphabet[0]
	for i := 0; i < len(data); i++ {
		if data[i] == firstChar {
			leadingZeros++
		} else {
			break
		}
	}

	digits := make([]int, 0, len(data)-leadingZeros)
	for i := leadingZeros; i < len(data); i++ {
		digits = append(digits, e.decodeMap[data[i]])
	}

	result := make([]int, 0)
	for len(digits) > 0 {
		carry := 0
		newDigits := make([]int, 0)

		for _, digit := range digits {
			carry = carry*e.base + digit
			if carry >= 256 || len(newDigits) > 0 {
				newDigits = append(newDigits, carry/256)
			}
			carry = carry % 256
		}

		result = append(result, carry)
		digits = newDigits
	}
	slices.Reverse(result)

	finalResult := make([]byte, leadingZeros+len(result))
	for i := 0; i < len(result); i++ {
		finalResult[leadingZeros+i] = byte(result[i])
	}

	return finalResult, nil
}
