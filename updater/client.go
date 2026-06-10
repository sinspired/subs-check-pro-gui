package updater

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const GhProxyBase = "https://proxy.linkpc.dpdns.org/"

type proxyTransport struct {
	base http.RoundTripper
}

func newProxyTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &proxyTransport{base: base}
}

func (t *proxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if shouldProxy(req.URL) {
		if proxied := proxyURL(req.URL); proxied != nil {
			newReq := req.Clone(req.Context())
			newReq.URL = proxied
			newReq.Host = proxied.Host
			return t.base.RoundTrip(newReq)
		}
	}
	return t.base.RoundTrip(req)
}

func shouldProxy(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := strings.ToLower(u.Host)
	switch {
	case host == "github.com" && strings.Contains(u.Path, "/releases/download/"):
		return true
	case host == "api.github.com":
		return true
	case host == "objects.githubusercontent.com":
		return true
	default:
		return false
	}
}

func proxyURL(original *url.URL) *url.URL {
	u, err := url.Parse(GhProxyBase + original.String())
	if err != nil {
		return nil
	}
	return u
}

// NewHTTPClient 构造用于 wails updater github.Config.HTTPClient 的客户端：
// 携带代理 transport，且不设置 client 级超时（避免大文件下载被截断）。
func NewHTTPClient() *http.Client {
	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       300 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: newProxyTransport(base),
		Timeout:   0,
	}
}