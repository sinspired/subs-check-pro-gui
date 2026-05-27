//go:build windows || darwin

package frontend

import (
	"context"
	_ "embed"
	"log/slog"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// platformHasTray Windows/macOS 支持系统托盘。
const platformHasTray = true

//go:embed tray.ico
var trayIconPNG []byte

var firstHide = true

// startSysTray 启动系统托盘（阻塞，建议在 goroutine 中调用）。
// ctx 是 Wails 运行时 context，onQuit 在用户点击"退出"时调用。
func startSysTray(ctx context.Context, g *GuiApp, onQuit func()) {
	systray.Run(
		func() { onSysTrayReady(ctx, g, onQuit) },
		func() { slog.Info("系统托盘已退出") },
	)
}

func onSysTrayReady(ctx context.Context, g *GuiApp, onQuit func()) {
	systray.SetIcon(trayIconPNG)
	systray.SetTitle("Subs Check Pro")
	systray.SetTooltip("Subs Check Pro — 订阅检测工具")

	mShow := systray.AddMenuItem("显示窗口", "恢复主界面")
	mHide := systray.AddMenuItem("隐藏窗口", "最小化到托盘")
	systray.AddSeparator()

	// 开机自启菜单项（带状态标记）
	mAutostart := systray.AddMenuItem("开机自启", "设置/取消开机启动")
	updateAutostartMenu(mAutostart)

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 Subs Check Pro")

	for {
		select {
		case <-mShow.ClickedCh:
			runtime.WindowShow(ctx)
			runtime.WindowSetAlwaysOnTop(ctx, true)
			runtime.WindowSetAlwaysOnTop(ctx, false)

		case <-mHide.ClickedCh:
			runtime.WindowHide(ctx)

		case <-mAutostart.ClickedCh:
			current := isAutostartEnabled()
			if err := setAutostart(!current); err != nil {
				slog.Error("切换开机自启失败", "error", err)
			}
			updateAutostartMenu(mAutostart)

		case <-mQuit.ClickedCh:
			systray.Quit()
			onQuit()
			return
		}
	}
}

func updateAutostartMenu(item *systray.MenuItem) {
	if isAutostartEnabled() {
		item.SetTitle("✓ 开机自启（已启用）")
		item.SetTooltip("点击取消开机自动启动")
	} else {
		item.SetTitle("  开机自启（已禁用）")
		item.SetTooltip("点击设置开机自动启动")
	}
}

// NotifyHideToTray 在首次隐藏到托盘时弹出一次提示（通过 Wails MessageDialog）。
// 之后调用只隐藏窗口，不再弹窗。
func NotifyHideToTray(ctx context.Context) {
	if firstHide {
		firstHide = false
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "已最小化到系统托盘",
			Message: "Subs Check Pro 仍在后台运行。\n右键点击任务栏托盘图标可以显示窗口或退出程序。",
		})
	}
}
