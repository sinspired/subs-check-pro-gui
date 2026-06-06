package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sinspired/subs-check-pro/v2/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// wailsOrigins 是 Wails webview 在各平台发出请求时携带的 Origin 头。
// WebView2 (Windows): http://wails.localhost
// WebKitGTK (Linux):  wails://
// WKWebView (macOS):  wails://
var wailsOrigins = map[string]bool{
	"http://wails.localhost": true,
	"wails://":               true,
}

// guiCORSMiddleware 为 Wails webview 的跨域请求添加必要的 CORS 响应头。
// 仅允许已知的 Wails origin，不对外开放，安全性不受影响。
func guiCORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if wailsOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-API-Key, Authorization")
			c.Header("Access-Control-Max-Age", "86400")
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}

// registerGuiRoutes 注册所有 /gui/* 辅助路由，并补全 CORS 中间件。
// 统一入口，便于在 setup.go 和 CompleteInit 中复用。
//
// ⚠️ CORS 修复说明：
// guiCORSMiddleware 之前已定义但从未注册，导致 Wails webview 从
// http://wails.localhost（Windows）/ wails://localhost（macOS）
// 直接向 http://127.0.0.1:PORT/api/* 发起绝对 URL 请求时被浏览器拦截。
// 此处 router.Use() 全局注册，确保所有路由均返回正确的 CORS 响应头。
// 主要修复手段是前端改用相对路径（走 Wails 资产代理），此处作防御性补充。
func registerGuiRoutes(router *gin.Engine) {
	if router == nil {
		slog.Warn("HTTP 服务未启动（端口冲突），跳过 /gui/* 路由注册")
		return
	}
	// 补全 CORS 中间件注册（之前定义了但从未 Use，导致 Wails webview 跨域请求失败）
	router.Use(guiCORSMiddleware())
	router.GET("/gui/enter", handleGuiEnter)
	router.GET("/gui/popup", handleGuiPopup)
	router.GET("/gui/back-to-login", handleGuiBackToLogin)
}

// registerGuiAutoLogin 向后兼容别名（setup.go 调用时使用）。
func registerGuiAutoLogin(router *gin.Engine) {
	registerGuiRoutes(router)
}

// ── /gui/enter ────────────────────────────────────────────────────────────────

// handleGuiEnter 一次性自动登录中转路由：验证 nonce → 写 session → 跳转 /admin。
func handleGuiEnter(c *gin.Context) {
	// 仅限 localhost 访问
	if !isLoopback(c) {
		c.String(http.StatusForbidden, "forbidden")
		return
	}

	nonce := c.Query("n")
	if nonce == "" {
		c.String(http.StatusBadRequest, "missing nonce")
		return
	}

	apiKey, remember, ok := consumeNonce(nonce)
	if !ok {
		c.String(http.StatusForbidden, "invalid or expired nonce")
		return
	}
	if apiKey != config.GlobalConfig.APIKey {
		c.String(http.StatusForbidden, "key mismatch")
		return
	}

	// 解析跳转目标：只允许以 "/" 开头的站内路径，防止开放重定向。
	redirect := c.Query("redirect")
	if redirect == "" || !strings.HasPrefix(redirect, "/") {
		redirect = "/admin"
	}

	c.Header("Cache-Control", "no-store, no-cache")
	c.Header("Content-Type", "text/html; charset=utf-8")

	var extraLS string
	if remember {
		extraLS = fmt.Sprintf(
			"try { localStorage.setItem('subscheck_api_key', %q); } catch(e) {}",
			apiKey,
		)
	}

	// 写入两个 storage key，兼容 admin.js（读 subscheck_session_key）
	// 和 analysis.js（读 subscheck_api_key）。
	c.String(http.StatusOK, fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="UTF-8">
<script>
(function(){
  try { sessionStorage.setItem('subscheck_session_key', %q); } catch(e) {}
  try { sessionStorage.setItem('subscheck_api_key', %q); } catch(e) {}
  %s
  window.location.replace(%q);
})();
</script>
</head><body></body></html>`, apiKey, apiKey, extraLS, redirect))
}

// ── /gui/popup ────────────────────────────────────────────────────────────────

// handleGuiPopup 接收来自 WebUI 注入脚本的"新窗口"请求，
// 在 Go 端创建无地址栏的 Wails 弹出窗口，风格与主窗口一致。
//
// 调用方：webUIWin 内注入的 JS 通过 fetch('/gui/popup?url=...&size=small') 触发。
// 安全策略：仅允许 localhost 访问，URL 只接受 http/https 协议。
func handleGuiPopup(c *gin.Context) {
	if !isLoopback(c) {
		c.String(http.StatusForbidden, "forbidden")
		return
	}

	rawURL := c.Query("url")
	if rawURL == "" {
		c.String(http.StatusBadRequest, "missing url")
		return
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		c.String(http.StatusBadRequest, "invalid url scheme")
		return
	}

	// 若目标是本机 Gin 服务的内部页面，通过 /gui/enter 中转自动写入
	// sessionStorage，使弹出窗口无需手动登录（与 OpenInternalPage 行为一致）。
	listenPort := strings.TrimPrefix(config.GlobalConfig.ListenPort, ":")
	if listenPort == "" {
		listenPort = "8199"
	}
	internalBase := "http://127.0.0.1:" + listenPort
	if strings.HasPrefix(rawURL, internalBase+"/") || rawURL == internalBase {
		// 提取路径 + query（保留 theme= 等参数）
		internalPath := strings.TrimPrefix(rawURL, internalBase)
		if internalPath == "" {
			internalPath = "/"
		}
		nonce := generateNonce(config.GlobalConfig.APIKey, false)
		rawURL = internalBase + "/gui/enter?n=" + nonce + "&redirect=" + url.QueryEscape(internalPath)
	}

	width, height := popupSize(c.Query("size"))

	// 先返回 200，让 JS fetch 立即结束，不阻塞页面
	c.String(http.StatusOK, "ok")

	// 在 Wails 主线程创建弹出窗口
	wailsApp := application.Get()
	if wailsApp == nil {
		slog.Error("/gui/popup: application.Get() returned nil") // 加这行
		return
	}

	capturedURL := rawURL
	slog.Info("/gui/popup: invoking popup", "url", capturedURL) // 加这行
	application.InvokeAsync(func() {
		// loading.html 中 hash 仅用于显示域名提示。
		// 实际导航由 Go 端 SetURL 完成，避免 JS 跨 origin 导航被 WebView 拦截。
		loadingURL := "/loading.html#" + capturedURL
		slog.Info("/gui/popup: inside InvokeAsync, creating window")
		popup := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:     "Subs Check Pro",
			Width:     width,
			Height:    height,
			MinWidth:  600,
			MinHeight: 400,
			URL:       loadingURL,
			Mac: application.MacWindow{
				InvisibleTitleBarHeight: 50,
				Backdrop:                application.MacBackdropTranslucent,
				TitleBar:                application.MacTitleBarHiddenInset,
			},
			DevToolsEnabled: true,
		})
		popup.Show()
		popup.Center()
		popup.Focus()
		popup.OpenDevTools()

		finalURL := capturedURL
		time.AfterFunc(300*time.Millisecond, func() {
			application.InvokeAsync(func() {
				popup.SetURL(finalURL)
			})
		})
	})
}

// popupSize 将尺寸名称映射为窗口宽高，与 OpenBrandURL 的规格保持一致。
func popupSize(size string) (width, height int) {
	switch size {
	case "extraLarge":
		return 1920, 1440
	case "large":
		return 1600, 1200
	case "medium":
		return 1200, 800
	case "small":
		return 720, 720
	case "tiny":
		return 600, 600
	case "wide":
		return 1600, 900
	default:
		return 1100, 750
	}
}

// ── 辅助 ──────────────────────────────────────────────────────────────────────

// handleGuiBackToLogin 接收来自 WebUI 的"返回登录窗口"请求。
// 仅限 localhost 访问，调用 globalGuiApp.BackToLogin() 切回登录小窗。
func handleGuiBackToLogin(c *gin.Context) {
	if !isLoopback(c) {
		c.String(http.StatusForbidden, "forbidden")
		return
	}
	c.String(http.StatusOK, "ok")
	if globalGuiApp != nil {
		application.InvokeAsync(func() {
			globalGuiApp.BackToLogin()
		})
	}
}

// isLoopback 检查请求是否来自本机回环地址。
func isLoopback(c *gin.Context) bool {
	remoteIP, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(remoteIP)
	return ip != nil && ip.IsLoopback()
}