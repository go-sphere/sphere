package cache

type Encoder interface {
	Marshal(val any) ([]byte, error)
}
type EncoderFunc func(val any) ([]byte, error)

func (e EncoderFunc) Marshal(val any) ([]byte, error) {
	return e(val)
}

type Decoder interface {
	Unmarshal(data []byte, val any) error
}

type DecoderFunc func(data []byte, val any) error

func (d DecoderFunc) Unmarshal(data []byte, val any) error {
	return d(data, val)
}
