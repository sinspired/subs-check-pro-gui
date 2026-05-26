package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sinspired/subs-check-pro/v2/config"
)

// registerGuiAutoLogin 注册 /gui/enter 一次性自动登录中转路由。
func registerGuiAutoLogin(router *gin.Engine) {
	if router == nil {
		slog.Warn("HTTP 服务未启动（端口冲突），跳过 /gui/enter 路由注册")
		return
	}
	router.GET("/gui/enter", handleGuiEnter)
}

func handleGuiEnter(c *gin.Context) {
	// 仅限 localhost 访问
	remoteIP, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		c.String(http.StatusForbidden, "forbidden")
		return
	}
	if ip := net.ParseIP(remoteIP); ip == nil || !ip.IsLoopback() {
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
