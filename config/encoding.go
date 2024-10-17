package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"path/filepath"
)

type Encoder interface {
	Encode(v interface{}) (err error)
}

type Decoder interface {
	Decode(v interface{}) error
}

type Extension string

const (
	ExtensionJSON Extension = "json"
	ExtensionYAML Extension = "yaml"
)

func Ext(path string) Extension {
	ext := filepath.Ext(path)
	if len(ext) > 0 && ext[0] == '.' {
		ext = ext[1:]
	}
	switch ext {
	case "json":
		return ExtensionJSON
	case "yaml", "yml":
		return ExtensionYAML
	default:
		return ""
	}
}

func NewDecoder(t Extension, r io.Reader) Decoder {
	switch t {
	case ExtensionYAML:
		return yaml.NewDecoder(r)
	case ExtensionJSON:
		return json.NewDecoder(r)
	default:
		return nil
	}
}

func NewEncoder(t Extension, w io.Writer) Encoder {
	switch t {
	case ExtensionYAML:
		return yaml.NewEncoder(w)
	case ExtensionJSON:
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder
	default:
		return nil
	}
}

var (
	ErrUnknownCoderType = fmt.Errorf("unknown coder type")
)

func Marshal(t Extension, v interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	encoder := NewEncoder(t, buf)
	if encoder == nil {
		return nil, ErrUnknownCoderType
	}
	err := encoder.Encode(v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Unmarshal(t Extension, data []byte, v interface{}) error {
	decoder := NewDecoder(t, bytes.NewReader(data))
	if decoder == nil {
		return ErrUnknownCoderType
	}
	return decoder.Decode(v)
}
