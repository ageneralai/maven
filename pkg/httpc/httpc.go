// Package httpc provides a shared HTTP client factory for proxy-aware egress.
package httpc

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ClientFromProxy returns an *http.Client whose transport uses the given proxy URL.
// If proxyURL is empty, http.DefaultClient is returned (HTTPS_PROXY env applies).
// Supported schemes: http, https, socks5, socks5h.
func ClientFromProxy(proxyURL string) (*http.Client, error) {
	if strings.TrimSpace(proxyURL) == "" {
		return http.DefaultClient, nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("httpc: invalid proxy URL %q: %w", proxyURL, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("httpc: unsupported proxy scheme %q", u.Scheme)
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(u),
		},
	}, nil
}
