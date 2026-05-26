package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"
	mihomoLog "github.com/metacubex/mihomo/log"
	"github.com/sinspired/subs-check-pro/v2/app"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
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
	fileHandler := tint.NewHandler(fileLogger, &tint.Options{
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
