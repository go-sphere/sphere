package codec

// Encoder defines the interface for encoding values into byte slices.
// It provides a generic way to serialize any data type to bytes.
type Encoder interface {
	// Marshal encodes the given value into a byte slice.
	// Returns an error if the encoding fails.
	Marshal(val any) ([]byte, error)
}

// EncoderFunc is a function adapter that implements the Encoder interface.
// It allows standalone functions to be used as Encoders.
type EncoderFunc func(val any) ([]byte, error)

// Marshal implements the Encoder interface by calling the function itself.
func (e EncoderFunc) Marshal(val any) ([]byte, error) {
	return e(val)
}

// Decoder defines the interface for decoding byte slices into values.
// It provides a generic way to deserialize bytes into any data type.
type Decoder interface {
	// Unmarshal decodes the given byte slice into the provided value.
	// The val parameter must be a pointer to the target type.
	// Returns an error if the decoding fails.
	Unmarshal(data []byte, val any) error
}

// DecoderFunc is a function adapter that implements the Decoder interface.
// It allows standalone functions to be used as Decoders.
type DecoderFunc func(data []byte, val any) error

// Unmarshal implements the Decoder interface by calling the function itself.
func (d DecoderFunc) Unmarshal(data []byte, val any) error {
	return d(data, val)
}

// Codec combines both encoding and decoding capabilities in a single interface.
// It's useful for components that need bidirectional serialization support.
type Codec interface {
	Encoder
	Decoder
}
