package main

import (
	"errors"
	"log/slog"
	"os"

	"github.com/sinspired/subs-check-pro/v2/app"
)

// setupApp 完成业务层初始化，返回三个值供 main() 使用：
//   - coreApp   核心应用实例（用于 Shutdown）
//   - guiApp    Wails 绑定实例（注入窗口引用后传给 Services）
//   - appInitOK 标记业务是否成功启动（退出时决定是否调用 Shutdown）
func setupApp() (*app.App, *GuiApp, bool) {
	os.Setenv("START_FROM_GUI", "1")

	originVersion := getEnvOrDefault("ORIGIN_VERSION", "dev")
	version       := getEnvOrDefault("APP_VERSION", "dev")
	configPath    := getEnvOrDefault("CONFIG_PATH", "")

	guiApp  := &GuiApp{configPath: configPath}
	coreApp := app.New(originVersion, version, configPath)

	if err := coreApp.Initialize(); err != nil {
		if errors.Is(err, app.ErrFirstRun) {
			resolvedPath := coreApp.GetConfigPath()
			slog.Info("首次运行：config.yaml 已创建", "path", resolvedPath)
			os.Setenv("GUI_FIRST_RUN", "1")

			coreApp = app.New(originVersion, version, resolvedPath)
			if err2 := coreApp.Initialize(); err2 != nil {
				slog.Error("首次运行后重新初始化失败", "error", err2)
				os.Exit(1)
			}
			guiApp.configPath = resolvedPath
		} else {
			slog.Error("应用初始化失败，无法启动 GUI", "error", err)
			os.Exit(1)
		}
	} else {
		guiApp.configPath = coreApp.GetConfigPath()
	}

	if err := coreApp.EnsureRouter(); err != nil {
		slog.Error("HTTP 路由初始化失败", "error", err)
		os.Exit(1)
	}

	registerGuiAutoLogin(coreApp.GetRouter())

	go coreApp.Run()

	return coreApp, guiApp, true
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
