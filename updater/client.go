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

// directProbeTransport 专门用于"直连探测" objects.githubusercontent.com。
// 拨号/握手超时刻意设置得比 base 短很多（base 是 60s，是为了不掐断大文件
// 下载本身），这样如果网络确实无法直连该 CDN，能在几秒内快速失败并回退
// 到代理，而不是让用户干等 60 秒。一旦连接建立成功，后续下载数据不受此
// 超时影响（Timeout 仍是每次 Dial/TLS 握手的超时，不是整个请求耗时）。
var directProbeTransport http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   6 * time.Second,
		KeepAlive: 60 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          10,
	IdleConnTimeout:       300 * time.Second,
	TLSHandshakeTimeout:   6 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func (t *proxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := ""
	if req.URL != nil {
		host = strings.ToLower(req.URL.Host)
	}

	// objects.githubusercontent.com 是 GitHub Releases 资产真正所在的 CDN
	// （签名 URL，通常全球可直连、带宽远高于个人代理）。安装包本体（几十
	// 上百 MB）就是在这里下载的，如果强制走代理，相当于把大文件下载全部
	// 挤占到自建代理的带宽/执行时长限制里，这是"以前快、现在慢"最常见的
	// 原因。因此这里优先尝试直连，只有直连失败（例如网络确实无法访问该
	// CDN）时才回退到代理，兼顾速度与可用性。
	if host == "objects.githubusercontent.com" {
		if resp, err := directProbeTransport.RoundTrip(req.Clone(req.Context())); err == nil {
			return resp, nil
		}
		if proxied := proxyURL(req.URL); proxied != nil {
			newReq := req.Clone(req.Context())
			newReq.URL = proxied
			newReq.Host = proxied.Host
			return t.base.RoundTrip(newReq)
		}
	}

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

// shouldProxy 只应代理"体积小、但域名容易被墙/需要 token 提升速率限制"的
// 请求（跳转前的 github.com 页面、api.github.com 的 API 调用）。真正承载
// 大文件内容的 objects.githubusercontent.com 不在这里处理，而是在
// RoundTrip 里做"直连优先、失败回退"的特殊逻辑，避免大文件被迫走代理。
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