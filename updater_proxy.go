// updater_proxy.go
// 为 GitHub Release 下载请求添加 ghproxy.net 代理前缀，改善中国大陆下载速度。
//
// 工作原理：
//   - 拦截目标主机为 github.com 且路径含 /releases/download/ 的 HTTP 请求
//   - 将原始 URL 改写为 https://ghproxy.net/<原始URL>
//   - GitHub API 请求（api.github.com）保持直连，仅大文件下载走代理
//
// 使用方式（main.go 中，创建 github.Provider 之前调用）：
//
//	http.DefaultTransport = newGHProxyTransport(http.DefaultTransport)
package main

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const ghProxyBase = "https://proxy.linkpc.dpdns.org/"

// ghProxyTransport 实现 http.RoundTripper，对 GitHub Release 下载 URL 加代理前缀。
type ghProxyTransport struct {
	base http.RoundTripper
}

// newGHProxyTransport 创建 ghProxyTransport，base 为原始 Transport（通常为 http.DefaultTransport）。
func newGHProxyTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &ghProxyTransport{base: base}
}

// RoundTrip 执行请求：
//   - github.com/*/releases/download/* → 经 ghproxy.net 代理
//   - objects.githubusercontent.com      → GitHub Release CDN，也走代理
//   - 其他所有请求                        → 直连
func (t *ghProxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if shouldProxy(req.URL) {
		proxied := proxyURL(req.URL)
		if proxied != nil {
			// 克隆请求以避免修改原对象（http.Client 不允许修改已发出请求）
			newReq := req.Clone(req.Context())
			newReq.URL = proxied
			newReq.Host = proxied.Host
			return t.base.RoundTrip(newReq)
		}
	}
	return t.base.RoundTrip(req)
}

// shouldProxy 判断是否需要走 ghproxy：
//   - github.com 的 releases/download 路径（实际二进制下载）
//   - objects.githubusercontent.com（GitHub Release CDN 节点）
//
// GitHub API（api.github.com）不代理，保留直连速度和认证兼容性。
func shouldProxy(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := strings.ToLower(u.Host)
	switch {
	case host == "github.com" && strings.Contains(u.Path, "/releases/download/") ||  host == "api.github.com": 
		return true
	case host == "objects.githubusercontent.com":
		return true
	default:
		return false
	}
}

// proxyURL 将原始 URL 改写为 https://ghproxy.net/<原始URL>。
// 若解析失败则返回 nil（调用方回退到直连）。
func proxyURL(original *url.URL) *url.URL {
	// ghproxy.net 接受格式：https://ghproxy.net/https://github.com/...
	rewritten := ghProxyBase + original.String()
	u, err := url.Parse(rewritten)
	if err != nil {
		return nil
	}
	return u
}

// 新增函数（可放在 updater_proxy.go 末尾）
func buildUpdaterHTTPClient() *http.Client {
    baseTransport := &http.Transport{
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
        // ResponseHeaderTimeout 不设（0 = 无限制）——慢速代理不会因首字节延迟中断
    }
    return &http.Client{
        Transport: newGHProxyTransport(baseTransport),
        Timeout:   0, // 不设 client 超时，避免大文件下载被截断
    }
}