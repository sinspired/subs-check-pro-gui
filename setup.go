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
	GuiVersion = "dev"

	// Version 内核（subs-check-pro）版本号，由构建脚本通过 -ldflags 注入：
	//   -X main.Version=v2.5.4
	Version = "dev"

	// CurrentCommit 内核最新提交的前 7 位 SHA，由构建脚本通过 -ldflags 注入：
	//   -X main.CurrentCommit=7c23868
	CurrentCommit = "unknown"
)

// setupApp 完成前后端初始化
func setupApp() (*app.App, *GuiApp, bool) {
	os.Setenv("START_FROM_GUI", "1")

	coreApp := app.New(Version, Version+CurrentCommit, "")

	guiApp := &GuiApp{configPath: ""}
	guiApp.isFirstRun = false
	guiApp.backend = coreApp

	// 端口预检与首次运行探测
	if err := coreApp.InitConfigLoad(); err != nil {
		if errors.Is(err, app.ErrFirstRun) {
			guiApp.isFirstRun = true
		} else {
			slog.Error("配置加载失败", "error", err)
			os.Exit(1)
		}
	}

	// 端口冲突检测
	httpPortAvailable, subStorePortAvailable := coreApp.CheckPortConflict()
	if !httpPortAvailable || !subStorePortAvailable {
		guiApp.pendingInit = true
		return coreApp, guiApp, false
	}

	// 初始化后端应用
	if err := coreApp.Initialize(); err != nil {
		slog.Error("应用初始化失败，无法启动 GUI", "error", err)
		os.Exit(1)
	}

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
