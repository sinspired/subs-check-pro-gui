// main.go
package main

import (
	"embed"
	"log/slog"
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	ensureSingleInstance()

	coreApp, guiApp, appInitOK := setupApp()
	globalGuiApp = guiApp // 供 router handler 访问

	notifier := notifications.New()
	InitNotifier(notifier)

	wailsApp := application.New(application.Options{
		Name:        "Subs Check Pro",
		Description: "订阅检测桌面管理面板",
		Services: []application.Service{
			application.NewService(guiApp),
			application.NewService(notifier),
			application.NewService(&Notifier{}),
		},
		Assets: application.AssetOptions{
			Handler: newCombinedAssetHandler(guiApp.configPath, guiApp.GetListenPort),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	// ── Wails Updater：检查 GitHub Release 更新 ──────────────────────────────
	// GuiVersion 由 ldflags 注入（如 "v2.5.6"），updater 要求不带 v 前缀
	currentVer := strings.TrimPrefix(GuiVersion, "v")
	if currentVer == "" || currentVer == "dev" {
		currentVer = "0.0.0" // dev 模式不触发真实更新检查
	}
	ghProvider, ghErr := github.New(github.Config{
		Repository:    "sinspired/subs-check-pro-gui",
		ChecksumAsset: "SHA256SUMS", // Release 中与产物同级的校验文件
	})
	if ghErr != nil {
		slog.Warn("Updater: 初始化 GitHub provider 失败", "error", ghErr)
	} else {
		if err := wailsApp.Updater.Init(updater.Config{
			CurrentVersion: currentVer,
			Providers:      []updater.Provider{ghProvider},
		}); err != nil {
			slog.Warn("Updater: Init 失败", "error", err)
		} else {
			slog.Debug("Updater: 已初始化", "currentVersion", currentVer)
		}
	}
	// 将 updater 引用注入 guiApp，供 binding 和托盘菜单调用
	guiApp.updaterApp = wailsApp

	// ── 窗口1：登录小窗（加载 Wails 前端资产）────────────────────────────────
	loginWin := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "login",
		Title:         "Subs Check Pro",
		Width:         560,
		Height:        400,
		MinWidth:      540,
		MaxWidth:      600,
		MinHeight:     380,
		MaxHeight:     420,
		DisableResize: false,
		Frameless:     false,
		AlwaysOnTop:   false,
		URL:           "/",
		KeyBindings: map[string]func(window application.Window){
			"F1": func(window application.Window) {
				guiApp.OpenAboutWindow()
			},
		},
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 30,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		Windows: application.WindowsWindow{
			DisableIcon:             false,
			HiddenOnTaskbar:         false,
			EnableSwipeGestures:     false,
			GeneralAutofillEnabled:  true,
			PasswordAutosaveEnabled: true,
		},
	})

	// ── 窗口2：WebUI 大窗（加载外部 Gin 服务，初始隐藏）─────────────────────
	webUIWin := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "webui",
		Title:         "Subs Check Pro",
		Width:         1200,
		Height:        800,
		MinWidth:      800,
		MinHeight:     600,
		Hidden:        true,
		DisableResize: false,
		Frameless:     false,
		URL:           "about:blank",
		KeyBindings: map[string]func(window application.Window){
			"F1": func(window application.Window) {
				guiApp.OpenAboutWindow()
			},
		},
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		Windows: application.WindowsWindow{
			DisableIcon:             false,
			HiddenOnTaskbar:         false,
			EnableSwipeGestures:     true,
			GeneralAutofillEnabled:  true,
			PasswordAutosaveEnabled: true,
		},
	})

	// 将窗口引用注入 guiApp
	guiApp.loginWin = loginWin
	guiApp.webUIWin = webUIWin
	guiApp.window = loginWin // 兼容旧托盘引用
	guiApp.autostart = wailsApp.Autostart

	// ── 登录窗口关闭钩子：转发给前端 QuitDialog ──────────────────────────────
	loginWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		loginWin.EmitEvent("window:close-requested", nil)
	})

	// ── WebUI 窗口关闭钩子：隐藏到托盘 ───────────────────────────────────────
	webUIWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		hideWindow(webUIWin)
		e.Cancel()
		sendOSNotification(
			"Subs Check Pro 后台运行",
			"右键点击托盘图标，选择「立即退出」可关闭程序",
		)
	})

	// ── WebUI 窗口最小化：隐藏到托盘 ─────────────────────────────────────────
	webUIWin.OnWindowEvent(events.Common.WindowMinimise, func(e *application.WindowEvent) {
		webUIWin.Hide()
		windowVisible.Store(false)
		sendOSNotification("管理界面", "已最小化到系统托盘")
		slog.Debug("WebUI 窗口已最小化到系统托盘")
	})

	// ── 登录窗口最小化：同样隐藏到托盘 ──────────────────────────────────────
	loginWin.OnWindowEvent(events.Common.WindowMinimise, func(e *application.WindowEvent) {
		loginWin.Hide()
		windowVisible.Store(false)
		sendOSNotification("登录窗口", "已最小化到系统托盘")
		slog.Debug("登录窗口已最小化到系统托盘")
	})

	// 退出时统一清理
	wailsApp.OnShutdown(func() {
		slog.Info("GUI 程序正在退出，执行清理工作…")
		// 使用动态方法而非启动时快照的 appInitOK：
		// 端口冲突场景下 appInitOK==false，CompleteInit() 成功后仍不会更新，
		// 导致 coreApp.Shutdown() 被跳过，Sub-Store 进程残留。
		if guiApp.IsBackendReady() {
			if err := coreApp.Shutdown(); err != nil {
				slog.Error("关闭应用失败", "error", err)
			}
		}
		sendOSNotification("Subs Check Pro", "已关闭")
	})

	// 单实例唤醒：显示当前活跃窗口
	go func() {
		for range showSignalCh {
			slog.Debug("收到单实例唤醒信号，显示主窗口")
			guiApp.showActiveWindow()
		}
	}()

	onQuit := func() {
		wailsApp.Quit()
	}

	startSysTray(wailsApp, guiApp, coreApp, onQuit)

	slog.Debug("Wails 双窗口已启动", "appReady", appInitOK)

	if err := wailsApp.Run(); err != nil {
		slog.Error("Wails 运行失败", "error", err)
		os.Exit(1)
	}
}
