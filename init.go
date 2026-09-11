package main

import (
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/lmittmann/tint"
	mihomoLog "github.com/metacubex/mihomo/log"
	"github.com/sinspired/subs-check-pro/v3/app"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
	if runtime.GOOS == "linux" {
		// 1. 彻底关闭 WebKit 沙盒，解决 bwrap / dbus-proxy 权限拒绝导致的崩溃
		// (针对 Ubuntu 24.04 / Debian 12+ 的 AppArmor 限制)
		os.Setenv("WEBKIT_DISABLE_SANDBOX", "1")

		// 2. 禁用 WebKit 的 GPU 硬件渲染，强制使用软件渲染
		// 解决 libEGL warning: failed to open /dev/dri 权限不够的问题
		os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")

		// 3. (可选) 如果在 Wayland 下依然有奇怪的崩溃，可以解除注释下面这行，强制使用 X11 模式
		// os.Setenv("GDK_BACKEND", "x11")
	}

	// 依赖库日志静默
	if os.Getenv("MIHOMO_DEBUG") != "" {
		mihomoLog.SetLevel(mihomoLog.DEBUG)
	} else {
		mihomoLog.SetLevel(mihomoLog.SILENT)
	}

	logLevel := getLogLevelWails()

	// GUI 模式：只写文件日志，不输出到控制台（避免 Windows 弹黑窗）
	fileLogger := &lumberjack.Logger{
		Filename:   app.TempLog(),
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}
	fileHandler := tint.NewTextHandler(fileLogger, &tint.Options{
		Level:      logLevel,
		TimeFormat: "01-02 15:04:05",
		NoColor:    true,
	})
	slog.SetDefault(slog.New(fileHandler))
}

func getLogLevelWails() slog.Level {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
