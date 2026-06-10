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
	//   go build -ldflags "-X main.GuiVersion=$(git describe --tags --abbrev=0) -X main.Version=v2.5.4 -X main.CurrentCommit=7c23868"
	GuiVersion = "dev"
	// Version 内核（subs-check-pro）版本号
	Version = "dev"
	// CurrentCommit 内核最新提交的前 7 位 SHA
	CurrentCommit = "unknown"
)

// setupApp 仅完成轻量级初始化：设置环境变量、加载配置、创建 GuiApp 结构体。
func setupApp() (*app.App, *GuiApp) {
	os.Setenv("START_FROM_GUI", "1")

	coreApp := app.New(Version, Version+CurrentCommit, "")

	guiApp := &GuiApp{configPath: ""}
	guiApp.isFirstRun = false
	guiApp.backend = coreApp

	// 仅加载配置，不初始化后端
	if err := coreApp.InitConfigLoad(); err != nil {
		if errors.Is(err, app.ErrFirstRun) {
			guiApp.isFirstRun = true
		} else {
			slog.Error("配置加载失败", "error", err)
			os.Exit(1)
		}
	}

	guiApp.configPath = coreApp.GetConfigPath()

	return coreApp, guiApp
}

// startBackend 在 Wails 框架就绪（ApplicationStarted）后调用，完成后端完整初始化。
// 端口冲突时设置 pendingInit=true 并返回 false，等待用户通过 UI 解决后由 CompleteInit 续接。
func startBackend(coreApp *app.App, guiApp *GuiApp) bool {
	// 端口冲突检测
	httpPortAvailable, subStorePortAvailable := coreApp.CheckPortConflict()
	if !httpPortAvailable || !subStorePortAvailable {
		guiApp.pendingInit = true
		return false
	}

	// 初始化后端应用（会启动 Node 子进程等重量级操作）
	if err := coreApp.Initialize(); err != nil {
		slog.Error("应用初始化失败，无法启动 GUI", "error", err)
		os.Exit(1)
	}

	guiApp.configPath = coreApp.GetConfigPath()

	registerGuiRoutes(coreApp.GetRouter())

	// 注入系统通知回调：核心包检测完成 → Wails3 托盘通知
	utils.OSNotifyHook = func(title, body string) {
		sendOSNotification(title, body)
	}

	go coreApp.Run()

	return true
}
