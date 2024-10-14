package configs

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	cfg := NewEmptyConfig()
	elem := reflect.ValueOf(cfg).Elem()
	fields := make([]string, 0)
	for i := 0; i < elem.NumField(); i++ {
		fields = append(fields, fmt.Sprintf("\"%s\"", elem.Type().Field(i).Name))
	}
	fmt.Printf("\twire.FieldsOf(new(*Config), %s),\n", strings.Join(fields, ", "))
}

func TestLoadRemoteConfig(t *testing.T) {
	_ = os.Setenv("CONSUL_HTTP_TOKEN", "883a2512-18eb-fdc7-497e-cc0e27e4639d")
	remote := RemoteConfig{
		Provider:   "consul",
		Endpoint:   "localhost:8500",
		Path:       "go-base",
		ConfigType: "json",
		SecretKey:  "",
	}
	config, err := LoadRemoteConfig(&remote)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(config)
}
