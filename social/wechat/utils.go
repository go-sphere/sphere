package wechat

import "unicode/utf8"

type RequestOptions struct {
	retryable         bool
	reloadAccessToken bool
}

func newRequestOptions(options ...RequestOption) *RequestOptions {
	opts := &RequestOptions{
		retryable:         true,
		reloadAccessToken: false,
	}
	for _, opt := range options {
		opt(opts)
	}
	return opts
}

type RequestOption = func(*RequestOptions)

func WithRetryable(retryable bool) RequestOption {
	return func(opts *RequestOptions) {
		opts.retryable = retryable
	}
}

func WithReloadAccessToken(reload bool) RequestOption {
	return func(opts *RequestOptions) {
		opts.reloadAccessToken = reload
	}
}

func WithClone(opts *RequestOptions) RequestOption {
	return func(o *RequestOptions) {
		o.retryable = opts.retryable
		o.reloadAccessToken = opts.reloadAccessToken
	}
}

func TruncateString(s string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	truncated := ""
	count := 0
	for _, runeValue := range s {
		if count >= maxChars {
			break
		}
		truncated += string(runeValue)
		count++
	}
	return truncated
}
