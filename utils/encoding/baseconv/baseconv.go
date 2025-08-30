// Package baseconv provides customizable base encoding/decoding functionality for arbitrary alphabets.
// It supports both bitwise and mathematical encoding algorithms with optional padding,
// making it suitable for creating URL-safe identifiers, compact data representations,
// and custom encoding schemes.
package baseconv

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

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

	charMap := make(map[byte]bool)
	for i := 0; i < len(alphabet); i++ {
		char := alphabet[i]
		if charMap[char] {
			return nil, errors.New("alphabet contains duplicate characters")
		}
		charMap[char] = true
	}

	if padChar != 0 && charMap[padChar] {
		return nil, errors.New("padding character conflicts with alphabet")
	}

	decodeMap := make(map[byte]int)
	for i := 0; i < len(alphabet); i++ {
		decodeMap[alphabet[i]] = i
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

	bitsPerChar := math.Log2(float64(e.base))
	if bitsPerChar != float64(int(bitsPerChar)) {
		return e.encodeMathematical(data)
	}

	return e.encodeBitwise(data, int(bitsPerChar))
}

func (e *BaseEncoding) encodeBitwise(data []byte, bitsPerChar int) string {
	if len(data) == 0 {
		return ""
	}

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
	if len(data) == 0 {
		return ""
	}

	digits := make([]int, 0)

	temp := make([]int, len(data))
	for i, b := range data {
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

		digits = append([]int{carry}, digits...)
		temp = newTemp
	}

	leadingZeros := 0
	for _, b := range data {
		if b == 0 {
			leadingZeros++
		} else {
			break
		}
	}

	var result strings.Builder
	for i := 0; i < leadingZeros; i++ {
		result.WriteByte(e.alphabet[0])
	}
	for _, digit := range digits {
		result.WriteByte(e.alphabet[digit])
	}

	if result.Len() == 0 {
		result.WriteByte(e.alphabet[0])
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

	bitsPerChar := math.Log2(float64(e.base))
	if bitsPerChar != float64(int(bitsPerChar)) {
		return e.decodeMathematical(data)
	}

	return e.decodeBitwise(data, int(bitsPerChar))
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

	digits := make([]int, 0)
	for i := 0; i < len(data); i++ {
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

		result = append([]int{carry}, result...)
		digits = newDigits
	}

	finalResult := make([]byte, leadingZeros+len(result))
	for i := 0; i < len(result); i++ {
		finalResult[leadingZeros+i] = byte(result[i])
	}

	return finalResult, nil
}
