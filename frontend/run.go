// Package frontend: run.go
// Run() 是 Wails GUI 的唯一入口，由 main_wails.go 调用。
package frontend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sinspired/subs-check-pro/v2/app"
	"github.com/sinspired/subs-check-pro/v2/config"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func Run() {
	os.Setenv("START_FROM_GUI", "1")

	originVersion := getEnvOrDefault("ORIGIN_VERSION", "dev")
	version := getEnvOrDefault("APP_VERSION", "dev")
	configPath := getEnvOrDefault("CONFIG_PATH", "")

	guiApp := &GuiApp{configPath: configPath}
	application := app.New(originVersion, version, configPath)

	// ─── 应用初始化 ────────────────────────────────────────────────────────────
	appInitOK := false
	if err := application.Initialize(); err != nil {
		if errors.Is(err, app.ErrFirstRun) {
			resolvedPath := application.GetConfigPath()
			slog.Info("首次运行：config.yaml 已创建", "path", resolvedPath)
			os.Setenv("GUI_FIRST_RUN", "1")

			application = app.New(originVersion, version, resolvedPath)
			if err2 := application.Initialize(); err2 != nil {
				slog.Error("首次运行后重新初始化失败", "error", err2)
				os.Exit(1)
			}
			guiApp.configPath = resolvedPath
		} else {
			slog.Error("应用初始化失败，无法启动 GUI", "error", err)
			os.Exit(1)
		}
	} else {
		guiApp.configPath = application.GetConfigPath()
	}

	if err := application.EnsureRouter(); err != nil {
		slog.Error("HTTP 路由初始化失败", "error", err)
		os.Exit(1)
	}

	// ─── 注册 GUI 专属路由 ─────────────────────────────────────────────────────
	// Issue #5：使用 nonce 替代明文 apiKey，避免 token 出现在 URL 中
	registerGuiAutoLogin(application.GetRouter())

	go application.Run()
	appInitOK = true

	// ─── 系统托盘（Issue #6）─────────────────────────────────────────────────
	// 托盘需要在 Wails OnStartup 之后才有 ctx，使用 channel 传递
	trayCtx := make(chan context.Context, 1)

	// 启动系统托盘
	go func() {
		slog.Info("等待 Wails Context 以启动系统托盘...")
		ctx := <-trayCtx
		startSysTray(ctx, guiApp, func() {
			// 彻底退出
			if appInitOK {
				_ = application.Shutdown()
			}
			os.Exit(0)
		})
	}()

	// ─── 启动 Wails 窗口 ──────────────────────────────────────────────────────
	err := wails.Run(&options.App{
		Title:         "Subs Check Pro",
		Width:         500,
		Height:        470, // 略高以容纳新增控件，无滚动条
		MinWidth:      460,
		MinHeight:     420,
		DisableResize: false,
		Frameless:     false,

		Assets: assets,
		Bind:   []interface{}{guiApp},

		OnStartup: func(ctx context.Context) {
			guiApp.startup(ctx)
			trayCtx <- ctx // 这里写入后，上面的 goroutine 就会立刻收到并解阻
			slog.Info("Wails 登录窗口已启动", "appReady", appInitOK)
		},

		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			runtime.WindowHide(ctx)
			go NotifyHideToTray(ctx)
			return true
		},

		OnShutdown: func(_ context.Context) {
			if appInitOK {
				if err := application.Shutdown(); err != nil {
					slog.Error("关闭应用失败", "error", err)
				}
			}
		},

		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisablePinchZoom:     true,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
		},
		Linux: &linux.Options{
			ProgramName: "subs-check-pro",
		},
	})

	if err != nil {
		slog.Error("Wails 运行失败", "error", err)
		os.Exit(1)
	}
}

// registerGuiAutoLogin 注册 /gui/enter 一次性自动登录中转路由。
//
// Issue #5 改进：不再接受明文 ?t=<apiKey>，改为接受 ?n=<nonce>。
// nonce 由 Wails 前端通过 GetEnterNonce() 方法获取，server 侧单次有效、30 秒过期。
// apiKey 只存在于服务器内存中，不再出现在 URL、浏览器历史或代理日志里。
//
// Issue #1 改进：支持 ?remember=1 参数，决定写 localStorage 还是 sessionStorage。
// - remember=0（默认）：只写 sessionStorage（本次 tab 有效，不跨会话）
// - remember=1：同时写 localStorage（跨会话持久化）
func registerGuiAutoLogin(router *gin.Engine) {
	if router == nil {
		// HTTP 服务因端口冲突未能启动，/gui/enter 路由无法注册。
		// GUI 窗口仍会启动并向用户展示端口冲突提示。
		slog.Warn("HTTP 服务未启动（端口冲突），跳过 /gui/enter 路由注册")
		return
	}
	router.GET("/gui/enter", func(c *gin.Context) {
		// 仅限 localhost 访问
		remoteIP, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			c.String(http.StatusForbidden, "forbidden")
			return
		}
		ip := net.ParseIP(remoteIP)
		if ip == nil || !ip.IsLoopback() {
			c.String(http.StatusForbidden, "forbidden")
			return
		}

		nonce := c.Query("n")
		if nonce == "" {
			c.String(http.StatusBadRequest, "missing nonce key")
			return
		}

		apiKey, remember, ok := consumeNonce(nonce)
		if !ok {
			c.String(http.StatusForbidden, "invalid or expired nonce key")
			return
		}

		// 额外校验：nonce 解出的 key 应与当前配置一致
		if apiKey != config.GlobalConfig.APIKey {
			c.String(http.StatusForbidden, "key mismatch")
			return
		}

		c.Header("Cache-Control", "no-store, no-cache")
		c.Header("Content-Type", "text/html; charset=utf-8")

		// Issue #1：根据 remember 决定写 sessionStorage 还是同时写 localStorage
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
	})
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
