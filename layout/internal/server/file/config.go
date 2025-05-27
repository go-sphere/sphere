package file

type HTTPConfig struct {
	Address string   `json:"address" yaml:"address"`
	Cors    []string `json:"cors" yaml:"cors"`
}

type Config struct {
	HTTP HTTPConfig `json:"http" yaml:"http"`
}
