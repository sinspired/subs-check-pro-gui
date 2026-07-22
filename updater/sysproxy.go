package updater

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// sysProxyAttemptTimeout 是第 1、2 层（系统代理相关）各自的连接/握手超时。
// 这两层只是"试一下走不走得通"，走不通就赶紧进入下一层，不能占用太多
// 整体下载时间预算（预算见 MaxDownloadDuration）。第 3 层作为最终兜底，
// 不再额外加短超时，只受调用方传入的 context 和 speedGuardTransport 约束。
const sysProxyAttemptTimeout = 8 * time.Second

// newSysProxyBaseTransport 通过系统代理（http.ProxyFromEnvironment 读取
// utils.GetSysProxy() 已设置好的 HTTP_PROXY/HTTPS_PROXY）转发请求。流量已
// 交给本地代理软件处理，DNS 污染、TLS 指纹限速都是代理软件自己要解决的
// 问题，这里不做额外伪装。
func newSysProxyBaseTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       300 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// fallbackTransport 按三层顺序尝试下载：
//  1. 系统代理可用 → 系统代理 + 原始地址（直连 GitHub，走本地代理软件）
//  2. 上一步失败/超时 → 系统代理 + GitHub 代理前缀
//  3. 系统代理不可用，或以上两层都失败 → DoH+utls + GitHub 代理前缀
//
// 只安全用于无请求体的 GET 场景（Check/Download 均是 GET）：原始 req 在
// 前两层失败后仍未被消费，可以原样交给下一层重试。
type fallbackTransport struct {
	sysProxyAvailable bool
	sysProxyDirect    http.RoundTripper // 第 1 层：系统代理 + 原始地址
	sysProxyGh        http.RoundTripper // 第 2 层：系统代理 + GitHub 代理前缀
	ghProxy           http.RoundTripper // 第 3 层：DoH+utls + GitHub 代理前缀（兜底）
}

func (t *fallbackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.sysProxyAvailable {
		if resp, err := t.tryWithTimeout(t.sysProxyDirect, req, sysProxyAttemptTimeout); err == nil {
			return resp, nil
		} else {
			slog.Warn("系统代理直连 GitHub 失败，尝试系统代理+GitHub代理前缀",
				"url", req.URL.String(), "error", err)
		}

		if resp, err := t.tryWithTimeout(t.sysProxyGh, req, sysProxyAttemptTimeout); err == nil {
			return resp, nil
		} else {
			slog.Warn("系统代理+GitHub代理前缀 也失败，回退到 DoH+utls 兜底方案",
				"url", req.URL.String(), "error", err)
		}
	}

	return t.ghProxy.RoundTrip(req)
}

// tryWithTimeout 用独立的短超时 context 包住单次尝试，避免某一层卡死拖垮
// 整体 fallback 流程。
//
// cancel 的生命周期：失败路径上立即调用 cancel 释放资源；成功路径上不能
// 立刻调用，否则响应体读取会被打断——改为通过 newCancelOnCloseReader 把
// cancel 传给响应体包装器，在调用方 Close() 时才真正触发，两条路径上
// cancel 最终都会被调用，避免 context 泄漏（对应 go vet lostcancel 检查）。
func (t *fallbackTransport) tryWithTimeout(rt http.RoundTripper, req *http.Request, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), timeout)
	r := req.Clone(ctx)
	resp, err := rt.RoundTrip(r)
	if err != nil {
		cancel()
		return nil, err
	}
	withBrowserHeaders(r)
	resp.Body = newCancelOnCloseReader(resp.Body, cancel)
	return resp, nil
}

// cancelOnCloseReader 包装 io.ReadCloser，在 Close() 时调用关联的
// context.CancelFunc，确保 tryWithTimeout 里创建的 context 无论下载成功
// 与否最终都会被释放。
type cancelOnCloseReader struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func newCancelOnCloseReader(rc io.ReadCloser, cancel context.CancelFunc) io.ReadCloser {
	return &cancelOnCloseReader{ReadCloser: rc, cancel: cancel}
}

func (r *cancelOnCloseReader) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}