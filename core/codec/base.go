package codec

import (
	"encoding/json"
	"errors"
)

type codec struct {
	encoder EncoderFunc
	decoder DecoderFunc
}

func (c *codec) Marshal(val any) ([]byte, error) {
	return c.encoder(val)
}

func (c *codec) Unmarshal(data []byte, val any) error {
	return c.decoder(data, val)
}

var (
	ErrInvalidType = errors.New("invalid type for codec operation")
	ErrNilPointer  = errors.New("nil pointer cannot be marshaled")
)

func StringCodec() Codec {
	return &codec{
		encoder: func(val any) ([]byte, error) {
			if str, ok := val.(string); ok {
				return []byte(str), nil
			}
			if strPtr, ok := val.(*string); ok {
				if strPtr == nil {
					return nil, ErrNilPointer
				}
				return []byte(*strPtr), nil
			}
			return nil, ErrInvalidType
		},
		decoder: func(data []byte, val any) error {
			if strPtr, ok := val.(*string); ok {
				*strPtr = string(data)
				return nil
			}
			return ErrInvalidType
		},
	}
}

func JsonCodec() Codec {
	return &codec{
		encoder: json.Marshal,
		decoder: json.Unmarshal,
	}
}
