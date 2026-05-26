package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sinspired/subs-check-pro/v2/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// registerGuiRoutes 注册所有 /gui/* 辅助路由。
// 统一入口，便于在 setup.go 和 CompleteInit 中复用。
func registerGuiRoutes(router *gin.Engine) {
	if router == nil {
		slog.Warn("HTTP 服务未启动（端口冲突），跳过 /gui/* 路由注册")
		return
	}
	router.GET("/gui/enter", handleGuiEnter)
	router.GET("/gui/popup", handleGuiPopup)
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

	c.Header("Cache-Control", "no-store, no-cache")
	c.Header("Content-Type", "text/html; charset=utf-8")

	var extraLS string
	if remember {
		extraLS = fmt.Sprintf(
			"try { localStorage.setItem('subscheck_api_key', %q); } catch(e) {}",
			apiKey,
		)
	}

	c.String(http.StatusOK, fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="UTF-8">
<script>
(function(){
  try { sessionStorage.setItem('subscheck_session_key', %q); } catch(e) {}
  %s
  window.location.replace('/admin');
})();
</script>
</head><body></body></html>`, apiKey, extraLS))
}

// ── /gui/popup ────────────────────────────────────────────────────────────────

// handleGuiPopup 接收来自 WebUI 注入脚本的"新窗口"请求，
// 在 Go 端创建无地址栏的 Wails 弹出窗口，风格与主窗口一致。
//
// 调用方：webUIWin 内注入的 JS 通过 fetch('/gui/popup?url=...') 触发。
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

	// 先返回 200，让 JS fetch 立即结束，不阻塞页面
	c.String(http.StatusOK, "ok")

	// 在 Wails 主线程创建弹出窗口
	wailsApp := application.Get()
	if wailsApp == nil {
		return
	}
	// application.InvokeAsync 是 Wails v3 中在主线程执行函数的正确方式
	// wailsApp.InvokeOnMainThread 在 v3 中已不存在
	capturedURL := rawURL
	application.InvokeAsync(func() {
		popup := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:     "Subs Check Pro",
			Width:     1100,
			Height:    750,
			MinWidth:  600,
			MinHeight: 400,
			URL:       capturedURL,
			Mac: application.MacWindow{
				InvisibleTitleBarHeight: 50,
				Backdrop:                application.MacBackdropTranslucent,
				TitleBar:                application.MacTitleBarHiddenInset,
			},
		})
		popup.Show()
		popup.Center()
		popup.Focus()
	})
}

// ── 辅助 ──────────────────────────────────────────────────────────────────────

// isLoopback 检查请求是否来自本机回环地址。
func isLoopback(c *gin.Context) bool {
	remoteIP, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(remoteIP)
	return ip != nil && ip.IsLoopback()
}