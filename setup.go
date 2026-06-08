// setup.go
package main

import (
	"errors"
	"log/slog"
	"os"

	"github.com/sinspired/subs-check-pro/v2/app"
	"github.com/sinspired/subs-check-pro/v2/utils"
)

var (
	// GuiVersion 桌面客户端自身版本，由构建脚本通过 -ldflags 注入，无需手动修改。
	//
	// 注入方式（Taskfile 已自动处理，推送 git tag 后触发 CI）：
	//   go build -ldflags "-X main.GuiVersion=$(git describe --tags --abbrev=0)"
	//
	// 本地 `wails3 dev` 开发模式不注入 ldflags，显示 "dev" 属正常行为。
	// 如需本地测试版本显示，可手动执行：
	//   GUI_VERSION=v1.2.0 task build
	GuiVersion = "dev"

	// Version 内核（subs-check-pro）版本号，由构建脚本通过 -ldflags 注入：
	//   -X main.Version=v2.5.4
	// 取自 github.com/sinspired/subs-check-pro 最新 Release 标签。
	Version = "dev"

	// CurrentCommit 内核最新提交的前 7 位 SHA，由构建脚本通过 -ldflags 注入：
	//   -X main.CurrentCommit=7c23868
	// 取自 github.com/sinspired/subs-check-pro 主分支最新提交。
	CurrentCommit = "unknown"
)

// setupApp 完成业务层初始化，返回三个值供 main() 使用：
//
//   - coreApp   核心应用实例（用于 Shutdown）
//
//   - guiApp    Wails 绑定实例（注入窗口引用后传给 Services）
//
//   - appInitOK 标记业务是否成功启动（退出时决定是否调用 Shutdown）
//
//     1. 先读取配置文件中的端口信息，检测端口是否可用。
//     2. 若端口冲突 → 不调用 coreApp.Initialize()，将 appInitOK=false 返回给 main；
//     Wails 窗口启动后前端读取 guiApp.HasPendingInit()==true，展示端口冲突界面；
//     用户修改端口并点击"应用"后，前端调用 GuiApp.CompleteInit() 完成初始化。
//     3. 若端口正常 → 按原有流程调用 Initialize() + EnsureRouter() + Run()。
func setupApp() (*app.App, *GuiApp, bool) {
	os.Setenv("START_FROM_GUI", "1")

	originVersion := getEnvOrDefault("ORIGIN_VERSION", "dev")
	version := getEnvOrDefault("APP_VERSION", "dev")
	configPath := getEnvOrDefault("CONFIG_PATH", "")

	guiApp := &GuiApp{configPath: configPath}
	coreApp := app.New(Version, Version+CurrentCommit, configPath)

	// 端口预检
	if err := coreApp.InitConfigLoad(); err != nil {
		if !errors.Is(err, app.ErrFirstRun) {
			slog.Error("配置加载失败", "error", err)
			os.Exit(1)
		}else{
			os.Setenv("GUI_FIRST_RUN", "1")
		}
	}

	httpPortAvailable, subStorePortAvailable := coreApp.CheckPortConflict()

	if !httpPortAvailable || !subStorePortAvailable {
		if !httpPortAvailable {
			slog.Warn("检测到 HTTP 端口冲突")
		}
		if !subStorePortAvailable {
			slog.Warn("检测到 Sub Store 端口冲突")
		}
		guiApp.pendingInit = true
		guiApp.configPath = coreApp.GetConfigPath()
		guiApp.backend = coreApp
		return coreApp, guiApp, false
	}

	// 初始化
	if err := coreApp.Initialize(); err != nil {
		sendOSNotification("首次运行", "setupApp")
		if errors.Is(err, app.ErrFirstRun) || os.Getenv("GUI_FIRST_RUN") == "1"{
			resolvedPath := coreApp.GetConfigPath()
			sendOSNotification("首次运行", "setupApp")
			slog.Info("哈哈哈首次运行：config.yaml 已创建", "path", resolvedPath)
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

	guiApp.backend = coreApp

	// 检查路由是否正确初始化并开启 WebUI
	if err := coreApp.EnsureRouterAndWebUI(); err != nil {
		slog.Error("HTTP 路由初始化失败", "error", err)
		os.Exit(1)
	}

	registerGuiRoutes(coreApp.GetRouter())

	// 注入系统通知回调：核心包检测完成 → Wails3 托盘通知
	utils.OSNotifyHook = func(title, body string) {
		sendOSNotification(title, body)
	}

	go coreApp.Run()

	return coreApp, guiApp, true
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
