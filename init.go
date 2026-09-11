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
		os.Setenv("WEBKIT_DISABLE_SANDBOX", "1")
		os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")
		// ⬇强行使用本地 VFS，绕过 Ubuntu 24.04 的 xdg-dbus-proxy 代理权限限制
		os.Setenv("GIO_USE_VFS", "local")
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
