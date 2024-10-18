package dash

type HTTPConfig struct {
	Address string   `json:"address" yaml:"address"`
	PProf   bool     `json:"pprof" yaml:"pprof"`
	Cors    []string `json:"cors" yaml:"cors"`
	Static  string   `json:"static" yaml:"static"`
}

type Config struct {
	JWT  string     `json:"jwt" yaml:"jwt"`
	HTTP HTTPConfig `json:"http" yaml:"http"`
}
