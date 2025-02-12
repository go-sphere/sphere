package dash

type HTTPConfig struct {
	Address string   `json:"address" yaml:"address"`
	PProf   bool     `json:"pprof" yaml:"pprof"`
	Cors    []string `json:"cors" yaml:"cors"`
	Static  string   `json:"static" yaml:"static"`
}

type Config struct {
	AuthJWT    string     `json:"auth_jwt" yaml:"auth_jwt"`
	RefreshJWT string     `json:"refresh_jwt" yaml:"refresh_jwt"`
	HTTP       HTTPConfig `json:"http" yaml:"http"`
}
