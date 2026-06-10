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

// singleInstanceKey 单实例 IPC 通道加密密钥，保证同一应用不同版本之间的实例可互相通信
var singleInstanceKey = [32]byte{
	0x73, 0x75, 0x62, 0x73, 0x2d, 0x63, 0x68, 0x65,
	0x63, 0x6b, 0x2d, 0x70, 0x72, 0x6f, 0x2d, 0x67,
	0x75, 0x69, 0x2d, 0x73, 0x69, 0x6e, 0x67, 0x6c,
	0x65, 0x2d, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x6e,
}

func main() {
	notifier := notifications.New()
	InitNotifier(notifier)

	// 轻量级初始化（Wails 就绪前）
	// 仅加载配置、创建结构体，不启动后端，规避 updater helper 实例
	// 提前拉起 Node 子进程后直接退出留下幽灵进程的问题。
	coreApp, guiApp := setupApp()
	globalGuiApp = guiApp

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

		// 第二次启动时：唤醒第一实例
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:      "com.sinspired.subs-check-pro-gui",
			EncryptionKey: singleInstanceKey,
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				slog.Debug("收到第二实例唤醒", "args", data.Args)
				application.InvokeAsync(func() {
					guiApp.showActiveWindow()
				})
			},
		},
	})

	// Wails Updater：检查 GitHub Release 更新
	currentVer := strings.TrimPrefix(GuiVersion, "v")
	if currentVer == "" || currentVer == "dev" {
		currentVer = "0.0.0"
	}

	ghProvider, ghErr := github.New(github.Config{
		Repository:    "sinspired/subs-check-pro-gui",
		ChecksumAsset: "SHA256SUMS",
		HTTPClient:    buildUpdaterHTTPClient(),
		// 自定义资产匹配器：Windows 优先选择纯二进制 .exe，跳过 setup 安装包
		AssetMatcher: func(req updater.CheckRequest, assets []github.ReleaseAsset) int {
			platform := strings.ToLower(req.Platform)
			arch := strings.ToLower(req.Arch)

			archAliases := []string{arch}
			switch arch {
			case "amd64":
				archAliases = append(archAliases, "x86_64", "x64")
			case "x64":
				archAliases = append(archAliases, "amd64", "x86_64")
			case "arm64":
				archAliases = append(archAliases, "aarch64")
			case "386":
				archAliases = append(archAliases, "i386", "x86", "ia32")
			}

			matchesPlatformArch := func(name string) bool {
				lower := strings.ToLower(name)
				if !strings.Contains(lower, platform) {
					return false
				}
				for _, a := range archAliases {
					if strings.Contains(lower, a) {
						return true
					}
				}
				return false
			}

			if platform == "windows" {
				for i, a := range assets {
					lower := strings.ToLower(a.Name)
					if matchesPlatformArch(a.Name) && strings.HasSuffix(lower, ".exe") && !strings.Contains(lower, "setup") {
						return i
					}
				}
				for i, a := range assets {
					lower := strings.ToLower(a.Name)
					if matchesPlatformArch(a.Name) && strings.HasSuffix(lower, ".exe") {
						return i
					}
				}
				return -1
			}

			for i, a := range assets {
				if matchesPlatformArch(a.Name) {
					return i
				}
			}
			return -1
		},
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
			initUpdaterDebugLog(wailsApp)
		}
	}
	guiApp.updaterApp = wailsApp

	// 登录窗，加载wails3前端资产
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

	// WebUI 大窗（加载 admin 页面，初始隐藏）
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

	// 注入窗口引用
	guiApp.loginWin = loginWin
	guiApp.webUIWin = webUIWin
	guiApp.autostart = wailsApp.Autostart

	// 窗口关闭/最小化拦截（窗口级别使用 RegisterHook）
	loginWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		loginWin.EmitEvent("window:close-requested", nil)
	})

	webUIWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		hideWindow(webUIWin)
		e.Cancel()
		sendOSNotification(
			"Subs Check Pro 后台运行",
			"右键点击托盘图标，选择「立即退出」可关闭程序",
		)
	})

	webUIWin.OnWindowEvent(events.Common.WindowMinimise, func(e *application.WindowEvent) {
		webUIWin.Hide()
		windowVisible.Store(false)
		sendOSNotification("管理界面", "已最小化到系统托盘")
	})

	loginWin.OnWindowEvent(events.Common.WindowMinimise, func(e *application.WindowEvent) {
		loginWin.Hide()
		windowVisible.Store(false)
		sendOSNotification("登录窗口", "已最小化到系统托盘")
	})

	// 退出生命周期清理
	wailsApp.OnShutdown(func() {
		slog.Info("GUI 程序正在退出，执行清理工作…")
		if guiApp.IsBackendReady() {
			if err := coreApp.Shutdown(); err != nil {
				slog.Error("关闭应用失败", "error", err)
			}
		}
		sendOSNotification("Subs Check Pro", "已关闭")
	})

	onQuit := func() { wailsApp.Quit() }
	startSysTray(wailsApp, guiApp, coreApp, onQuit)

	// Wails 就绪后才启动后端
	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		appInitOK := startBackend(coreApp, guiApp)
		slog.Debug("Wails 就绪，后端初始化完成", "appReady", appInitOK)
	})

	if err := wailsApp.Run(); err != nil {
		slog.Error("Wails 运行失败", "error", err)
		os.Exit(1)
	}
}
