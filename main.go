package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {
	// 若检测到已有实例，向其发送唤醒信号后退出本进程。
	ensureSingleInstance()

	coreApp, guiApp, appInitOK := setupApp()

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	wailsApp := application.New(application.Options{
		Name:        "Subs Check Pro",
		Description: "订阅检测桌面管理面板",
		Services: []application.Service{
			application.NewService(guiApp),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	win := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "main",
		Title:         "Subs Check Pro",
		Width:         500,
		Height:        470,
		MinWidth:      460,
		MinHeight:     420,
		DisableResize: false,
		Frameless:     false,
		URL:           "/",
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

	guiApp.window = win

	// 窗口关闭 → 最小化到托盘
	win.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		win.Hide()
		go NotifyHideToTray()
	})

	// 后台协程：每秒发送一次事件，供前端定时器订阅（例如更新托盘时间显示）。
	go func() {
		for range showSignalCh {
			slog.Info("收到单实例唤醒信号，显示主窗口")
			showWindow(win)
		}
	}()

	// 系统托盘
	startSysTray(wailsApp, win, guiApp, func() {
		if appInitOK {
			_ = coreApp.Shutdown()
		}
		os.Exit(0)
	})

	// 应用退出
	wailsApp.OnShutdown(func() {
		if appInitOK {
			if err := coreApp.Shutdown(); err != nil {
				slog.Error("关闭应用失败", "error", err)
			}
		}
	})

	slog.Info("Wails 登录窗口已启动", "appReady", appInitOK)

	// Run the application. This blocks until the application has been exited.
	if err := wailsApp.Run(); err != nil {
		slog.Error("Wails 运行失败", "error", err)
		os.Exit(1)
	}
}
