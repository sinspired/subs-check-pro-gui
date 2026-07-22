package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	utls "github.com/metacubex/utls"
	"github.com/sinspired/subs-check-pro/v2/config"
	"github.com/sinspired/subs-check-pro/v2/utils"
)

// GhProxyBase 是本项目自建的 CF-Proxy Worker 中转地址，作为 GithubProxyGroup
// 的固定候选之一参与测速打分，不再单独硬编码使用——最终使用哪个由
// utils.GetGhProxy() 的并发测速结果决定（见 resolveGhProxyBase）。
const GhProxyBase = "https://proxy.linkpc.dpdns.org/"

// MaxDownloadDuration 是整个更新下载流程能接受的最长耗时（130MB 安装包，
// 3-5 分钟内下不完就认为这条链路不可用）。调用方（触发 Updater.Download
// 的地方）应该用这个值构造 context.WithTimeout 传进来，而不是依赖这里的
// 客户端做整体超时——client.Timeout 会在下载到一半时把已收到的数据直接
// 截断报错，不适合大文件流式下载；由调用方在业务层面控制整体截止时间，
// 到期后可以提示用户"网络较差，建议稍后重试或手动下载"。
const MaxDownloadDuration = 5 * time.Minute

// minDownloadSpeedBytesPerSec / speedCheckGrace 用于"下载已开始但速度明显
// 不达标"时尽早失败，而不是傻等到 MaxDownloadDuration 超时才报错。
// 130MB / 5min ≈ 443KB/s，这里留一点余量，低于 300KB/s 持续一段时间就
// 判定为这条链路不可用。
const (
	minDownloadSpeedBytesPerSec = 300 * 1024 // 300KB/s
	speedCheckGrace             = 20 * time.Second
)

// ---------------------------------------------------------------------
// 三层问题，三层修复：
//
// 1) DNS 污染：本地/运营商 DNS 把域名解析到被限速/污染的地址。
//    → dohDialContext 接管域名解析，走 DoH 拿到未被劫持的真实 IP。
//
// 2) TLS 指纹限速：运营商/防火墙按 TLS ClientHello 指纹（JA3）做 QoS。
//    → dohDialTLSContext 用 utls 伪装成 Chrome 的 ClientHello 指纹。
//
// 3) 中转节点本身质量不稳（边缘节点到 GitHub CDN 的路径质量参差）。
//    → 不再写死单一 GhProxyBase，而是把它并入 GithubProxyGroup，交给
//      utils.GetGhProxy() 的并发测速+打分逻辑挑最优节点。
// ---------------------------------------------------------------------

var dohProviders = []string{
	"https://doh.pub/dns-query?name=%s&type=A",
	"https://dns.alidns.com/dns-query?name=%s&type=A",
}

var dohHTTPClient = &http.Client{Timeout: 3 * time.Second}

type dohResponse struct {
	Answer []struct {
		Type int    `json:"type"`
		Data string `json:"data"`
	} `json:"Answer"`
}

type dnsCacheEntry struct {
	ips     []string
	expires time.Time
}

var (
	dnsCacheMu sync.Mutex
	dnsCache   = map[string]dnsCacheEntry{}
)

func dohLookup(ctx context.Context, host string) []string {
	dnsCacheMu.Lock()
	if e, ok := dnsCache[host]; ok && time.Now().Before(e.expires) {
		dnsCacheMu.Unlock()
		return e.ips
	}
	dnsCacheMu.Unlock()

	for _, tmpl := range dohProviders {
		reqURL := fmt.Sprintf(tmpl, url.QueryEscape(host))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/dns-json")

		resp, err := dohHTTPClient.Do(req)
		if err != nil {
			continue
		}
		var parsed dohResponse
		decErr := json.NewDecoder(resp.Body).Decode(&parsed)
		resp.Body.Close()
		if decErr != nil {
			continue
		}

		var ips []string
		for _, a := range parsed.Answer {
			if a.Type == 1 {
				ips = append(ips, a.Data)
			}
		}
		if len(ips) > 0 {
			dnsCacheMu.Lock()
			dnsCache[host] = dnsCacheEntry{ips: ips, expires: time.Now().Add(5 * time.Minute)}
			dnsCacheMu.Unlock()
			return ips
		}
	}
	return nil
}

var systemDialer = &net.Dialer{
	Timeout:   10 * time.Second,
	KeepAlive: 60 * time.Second,
}

func dohDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return systemDialer.DialContext(ctx, network, addr)
	}
	if net.ParseIP(host) != nil {
		return systemDialer.DialContext(ctx, network, addr)
	}

	if ips := dohLookup(ctx, host); len(ips) > 0 {
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := systemDialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		if lastErr != nil {
			return nil, lastErr
		}
	}
	return systemDialer.DialContext(ctx, network, addr)
}

func dohDialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	rawConn, err := dohDialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		host = addr
	}

	uConn := utls.UClient(rawConn, &utls.Config{
		ServerName: host,
	}, utls.HelloChrome_Auto)

	if buildErr := uConn.BuildHandshakeState(); buildErr != nil {
		rawConn.Close()
		return nil, buildErr
	}
	for _, ext := range uConn.Extensions {
		if alpn, ok := ext.(*utls.ALPNExtension); ok {
			alpn.AlpnProtocols = []string{"http/1.1"}
		}
	}

	if hsErr := uConn.HandshakeContext(ctx); hsErr != nil {
		rawConn.Close()
		return nil, hsErr
	}
	return uConn, nil
}

// proxyTransport 把命中 shouldProxy 的请求改写为 ghProxyBase + 原始 URL。
// ghProxyBase 不再是全局常量，而是每次构造时传入——因为它现在是
// resolveGhProxyBase() 测速选出的结果，系统代理层和 DoH+utls 层可能拿到
// 同一个最优地址，但各自的底层 base RoundTripper（拨号方式）不同。
type proxyTransport struct {
	base        http.RoundTripper
	ghProxyBase string
}

func newProxyTransport(base http.RoundTripper, ghProxyBase string) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &proxyTransport{base: base, ghProxyBase: ghProxyBase}
}

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

func withBrowserHeaders(req *http.Request) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", browserUA)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "*/*")
	}
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	}
}

func (t *proxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if shouldProxy(req.URL) {
		if proxied := t.proxyURL(req.URL); proxied != nil {
			newReq := req.Clone(req.Context())
			newReq.URL = proxied
			newReq.Host = proxied.Host
			withBrowserHeaders(newReq)
			return t.base.RoundTrip(newReq)
		}
	}
	withBrowserHeaders(req)
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

func (t *proxyTransport) proxyURL(original *url.URL) *url.URL {
	u, err := url.Parse(t.ghProxyBase + original.String())
	if err != nil {
		return nil
	}
	return u
}

// resolveGhProxyBase 把 GhProxyBase 并入 config.GlobalConfig.GithubProxyGroup
// （去重后追加），交给 utils.GetGhProxy() 做并发测速+打分，选出当前实际
// 可用且最快的一个。选不出可用节点时兜底回退到 GhProxyBase 本身，保证
// 至少有一个地址可用（即便它当下也慢）。
func resolveGhProxyBase() string {
	base := config.GlobalConfig.GithubProxyGroup
	found := false
	for _, p := range base {
		if strings.TrimRight(p, "/") == strings.TrimRight(GhProxyBase, "/") {
			found = true
			break
		}
	}
	if !found {
		base = append(base, GhProxyBase)
		config.GlobalConfig.GithubProxyGroup = base
	}

	if utils.GetGhProxy() {
		best := config.GlobalConfig.GithubProxy
		if best != "" {
			slog.Info("已选定最优 GitHub 代理节点", "proxy", best)
			return best
		}
	}

	slog.Warn("GitHub 代理测速未选出可用节点，回退到默认地址", "proxy", GhProxyBase)
	return GhProxyBase
}

// speedGuardReader 包裹响应体，持续统计下载速度；经过 speedCheckGrace 的
// 预热期后（排开 TCP 慢启动、代理握手等初始抖动），若平均速度低于
// minDownloadSpeedBytesPerSec，主动中断并返回明确错误，而不是傻等到
// MaxDownloadDuration 超时——尤其对最后一层兜底方案有意义：与其耗光
// 5 分钟额度才失败，不如提前几十秒发现"这条链路今天就是不行"，把错误
// 尽快暴露给用户，提示改天重试或手动下载。
//
// 注意：这个中断发生在 RoundTrip 已经返回、调用方正在读 Body 的阶段，
// 无法再触发 fallbackTransport 切换到下一层（切层判断只在收到响应头之前
// 有效）。它的作用是"及时止损+给出清晰错误"，不是"自动换线路"。
type speedGuardReader struct {
	io.ReadCloser
	start          time.Time
	total          int64
	minBytesPerSec float64
	grace          time.Duration
	cancel         context.CancelFunc
	aborted        bool
}

func (r *speedGuardReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.total += int64(n)
		elapsed := time.Since(r.start)
		if elapsed > r.grace {
			speed := float64(r.total) / elapsed.Seconds()
			if speed < r.minBytesPerSec {
				r.aborted = true
				r.cancel()
				r.ReadCloser.Close()
				return n, fmt.Errorf(
					"下载速度过低（%.0f KB/s，低于 %.0f KB/s 阈值），已中止：建议稍后重试或手动下载",
					speed/1024, r.minBytesPerSec/1024,
				)
			}
		}
	}
	if err != nil && r.aborted {
		// 已经用自定义错误说明原因，避免上层再看到一个语义不明的
		// context canceled 覆盖掉真正原因。
		return n, nil
	}
	return n, err
}

func (r *speedGuardReader) Close() error {
	r.cancel()
	return r.ReadCloser.Close()
}

// speedGuardTransport 给任意 RoundTripper 包一层限速保护。
type speedGuardTransport struct {
	base http.RoundTripper
}

func (t *speedGuardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	r := req.Clone(ctx)
	resp, err := t.base.RoundTrip(r)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &speedGuardReader{
		ReadCloser:     resp.Body,
		start:          time.Now(),
		minBytesPerSec: minDownloadSpeedBytesPerSec,
		grace:          speedCheckGrace,
		cancel:         cancel,
	}
	return resp, nil
}

// NewHTTPClient 构造用于 wails updater github.Config.HTTPClient 的客户端。
//
// 请求按以下三层顺序尝试（见 fallbackTransport）：
//  1. 系统代理 + 原始 GitHub 地址
//  2. 系统代理 + GitHub 代理前缀（前一层连不通 GitHub 本身时用）
//  3. DoH 解析 + utls 伪装 Chrome 指纹 + GitHub 代理前缀（系统代理不可用，
//     或以上两层都失败时兜底；不提供纯直连 GitHub 选项）
//
// 每一层最终命中的代理前缀地址，都是从 GithubProxyGroup（含本项目自建的
// GhProxyBase）里并发测速选出的最优节点，而不是写死用 GhProxyBase。
//
// 调用方应该用 context.WithTimeout(ctx, updater.MaxDownloadDuration) 包一层
// 传给 Updater.Check/Download，作为整体下载耗时的硬性上限；本函数内部的
// speedGuardTransport 只负责"明显过慢时提前失败"，不替代整体超时控制。
func NewHTTPClient() *http.Client {
	ghProxyBase := resolveGhProxyBase()
	sysProxyOK := utils.GetSysProxy()

	ghBase := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dohDialContext,
		DialTLSContext:        dohDialTLSContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          10,
		IdleConnTimeout:       300 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	ft := &fallbackTransport{
		sysProxyAvailable: sysProxyOK,
		ghProxy:           newProxyTransport(ghBase, ghProxyBase), // 第 3 层
	}
	if sysProxyOK {
		sysBase := newSysProxyBaseTransport()
		ft.sysProxyDirect = sysBase                            // 第 1 层
		ft.sysProxyGh = newProxyTransport(sysBase, ghProxyBase) // 第 2 层
	}

	return &http.Client{
		Transport: &speedGuardTransport{base: ft},
		Timeout:   0, // 整体耗时上限由调用方通过 context 控制，见函数注释
	}
}