package parser

import (
	"net/http"
	"os"
	"time"
)

type RemoteConfig struct {
	URL     string            `json:"url" yaml:"url"`
	Method  string            `json:"method" yaml:"method"`
	Headers map[string]string `json:"headers" yaml:"headers"`
}

func Local[T any](path string) (*T, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := new(T)
	err = Unmarshal(Ext(path), file, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func Remote[T any](remote *RemoteConfig) (*T, error) {
	httpClient := http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest(remote.Method, remote.URL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range remote.Headers {
		req.Header.Add(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	config := new(T)
	decoder := NewDecoder(Ext(remote.URL), resp.Body)
	if decoder == nil {
		return nil, ErrUnknownCoderType
	}
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
