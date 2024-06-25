package request

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

func DefaultHttpClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

func URL(base string, query map[string]string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	params := url.Values{}
	for k, v := range query {
		params.Add(k, v)
	}
	baseURL.RawQuery = params.Encode()
	return baseURL.String(), nil
}

func GET[T any](url string) (*T, error) {
	return GETx[T](url, nil)
}

func GETx[T any](url string, reqModifier func(req *http.Request)) (*T, error) {
	client := DefaultHttpClient()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result T
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func POST[T any](url string, data any) (*T, error) {
	return POSTx[T](url, data, nil)
}

func POSTx[T any](url string, data any, reqModifier func(req *http.Request)) (*T, error) {
	client := DefaultHttpClient()
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json; charset=utf-8")
	if reqModifier != nil {
		reqModifier(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result T
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
