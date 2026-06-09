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

// singleInstanceKey 单实例 IPC 通道加密密钥。
// 固定写入源码，保证同一应用不同版本之间的实例可互相通信（勿随机生成）。
var singleInstanceKey = [32]byte{
    0x73, 0x75, 0x62, 0x73, 0x2d, 0x63, 0x68, 0x65,
    0x63, 0x6b, 0x2d, 0x70, 0x72, 0x6f, 0x2d, 0x67,
    0x75, 0x69, 0x2d, 0x73, 0x69, 0x6e, 0x67, 0x6c,
    0x65, 0x2d, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x6e,
}

func main() {
	notifier := notifications.New()
	InitNotifier(notifier)

	coreApp, guiApp, appInitOK := setupApp()
	globalGuiApp = guiApp // 供 router handler 访问

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

		// 第二次启动时：唤醒第一实例的当前活跃窗口（登录小窗 或 WebUI 大窗）。
		// application.InvokeAsync 确保窗口操作在 Wails 主线程执行。
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

	// ── Wails Updater：检查 GitHub Release 更新 ──────────────────────────────
	// GuiVersion 由 ldflags 注入（如 "v2.5.6"），updater 要求不带 v 前缀
	currentVer := strings.TrimPrefix(GuiVersion, "v")
	if currentVer == "" || currentVer == "dev" {
		currentVer = "0.0.0" // dev 模式不触发真实更新检查
	}

	// ── ghproxy 代理：为 GitHub Release 下载注入 Transport ──────────────────
	// 仅拦截 github.com/*/releases/download/* 及 objects.githubusercontent.com 请求，
	// GitHub API（api.github.com）保持直连。
	// 该 Transport 对整个进程生效，不影响其他 HTTP 请求（Gin、Sub-Store 等均走本地回环）。
	// http.DefaultTransport = newGHProxyTransport(http.DefaultTransport)

	ghProvider, ghErr := github.New(github.Config{
		Repository:    "sinspired/subs-check-pro-gui",
		ChecksumAsset: "SHA256SUMS", // Release 中与产物同级的校验文件
		HTTPClient:    buildUpdaterHTTPClient(),
		// 自定义资产匹配器：Windows 优先选择纯二进制 .exe，跳过 setup 安装包；
		// 其他平台保持与默认匹配器相同的 GOOS+GOARCH 子串逻辑。
		AssetMatcher: func(req updater.CheckRequest, assets []github.ReleaseAsset) int {
			platform := strings.ToLower(req.Platform)
			arch := strings.ToLower(req.Arch)

			// 与默认匹配器一致的 arch 别名表
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
				// 第一优先：匹配平台+架构且以 .exe 结尾、名称中不含 "setup" 的纯二进制
				for i, a := range assets {
					lower := strings.ToLower(a.Name)
					if matchesPlatformArch(a.Name) &&
						strings.HasSuffix(lower, ".exe") &&
						!strings.Contains(lower, "setup") {
						return i
					}
				}
				// 回退：任意匹配平台+架构的 .exe（含 setup）
				for i, a := range assets {
					lower := strings.ToLower(a.Name)
					if matchesPlatformArch(a.Name) && strings.HasSuffix(lower, ".exe") {
						return i
					}
				}
				return -1
			}

			// 非 Windows：标准 平台+架构 子串匹配
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
			initUpdaterDebugLog(wailsApp) // 启动文件日志监听
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
