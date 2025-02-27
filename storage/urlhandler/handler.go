package urlhandler

import (
	"fmt"
	"net/url"
	"strings"
)

var (
	ErrorNotVerifyHost = fmt.Errorf("not verify host")
)

type Handler struct {
	publicURLBase string
	publicURLHost string
}

func NewHandler(public string) (*Handler, error) {
	base, err := url.Parse(public)
	if err != nil {
		return nil, err
	}
	return &Handler{
		publicURLBase: base.String(),
		publicURLHost: base.Host,
	}, nil
}

func (n *Handler) hasHttpScheme(uri string) bool {
	return strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")
}

func (n *Handler) GenerateURL(key string) string {
	if key == "" {
		return ""
	}
	if n.hasHttpScheme(key) {
		return key
	}
	result, err := url.JoinPath(n.publicURLBase, key)
	if err != nil {
		return ""
	}
	return result
}

func (n *Handler) GenerateURLs(keys []string) []string {
	urls := make([]string, len(keys))
	for i, key := range keys {
		urls[i] = n.GenerateURL(key)
	}
	return urls
}

func (n *Handler) ExtractKeyFromURLWithMode(uri string, strict bool) (string, error) {
	if uri == "" {
		return "", nil
	}
	// 不是 http 或者 https 开头的直接返回
	if !n.hasHttpScheme(uri) {
		return strings.TrimPrefix(uri, "/"), nil
	}
	// 解析URL
	u, err := url.Parse(uri)
	if err != nil {
		return "", nil
	}
	if strict {
		// 不是以CDN域名开头的直接返回或者报错
		if u.Host != n.publicURLHost {
			return "", ErrorNotVerifyHost
		}
	}
	u.RawQuery = ""
	u.RawFragment = ""
	// 返回去掉base url的key
	return strings.TrimPrefix(strings.TrimPrefix(u.String(), n.publicURLBase), "/"), nil

}

func (n *Handler) ExtractKeyFromURL(uri string) string {
	key, _ := n.ExtractKeyFromURLWithMode(uri, true)
	return key
}
