package api

type HTTPConfig struct {
	Address string   `json:"address" yaml:"address"`
	Cors    []string `json:"cors" yaml:"cors"`
}

type Config struct {
	JWT  string     `json:"jwt" yaml:"jwt"`
	HTTP HTTPConfig `json:"http" yaml:"http"`
}
