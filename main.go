// main.go
package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
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
		},
		Assets: application.AssetOptions{
			Handler: newCombinedAssetHandler(guiApp.configPath, guiApp.GetListenPort),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

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
		URL:           "/",
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
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
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
			"Subs Check Pro 仍在后台运行",
			"右键点击托盘图标，选择「立即退出」可关闭程序",
		)
	})

	// ── WebUI 窗口最小化：隐藏到托盘 ─────────────────────────────────────────
	webUIWin.OnWindowEvent(events.Common.WindowMinimise, func(e *application.WindowEvent) {
		webUIWin.Hide()
		windowVisible.Store(false)
		sendOSNotification("Subs Check Pro", "已最小化到系统托盘\n单击托盘图标可恢复窗口")
		slog.Debug("WebUI 窗口已最小化到系统托盘")
	})

	// ── 登录窗口最小化：同样隐藏到托盘 ──────────────────────────────────────
	loginWin.OnWindowEvent(events.Common.WindowMinimise, func(e *application.WindowEvent) {
		loginWin.Hide()
		windowVisible.Store(false)
		sendOSNotification("Subs Check Pro", "已最小化到系统托盘\n单击托盘图标可恢复窗口")
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
		sendOSNotification("Subs Check Pro", "已退出")
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
