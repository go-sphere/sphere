package wechat

import "unicode/utf8"

type requestOptions struct {
	retryable         bool
	reloadAccessToken bool
}

func newRequestOptions(opts ...RequestOption) *requestOptions {
	defaults := &requestOptions{
		retryable:         true,
		reloadAccessToken: false,
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

type RequestOption = func(*requestOptions)

func WithRetryable(retryable bool) RequestOption {
	return func(opts *requestOptions) {
		opts.retryable = retryable
	}
}

func WithReloadAccessToken(reload bool) RequestOption {
	return func(opts *requestOptions) {
		opts.reloadAccessToken = reload
	}
}

func WithClone(opts *requestOptions) RequestOption {
	return func(o *requestOptions) {
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
