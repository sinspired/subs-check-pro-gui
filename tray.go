// Package main: tray.go
package main

import (
	_ "embed"
	"log/slog"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// trayIcon 嵌入托盘图标文件（ICO 格式，Windows/Linux 通用）。
// macOS 建议改用透明背景 PNG 并调用 SetTemplateIcon。
//
//go:embed tray.ico
var trayIcon []byte

// windowVisible 跟踪当前窗口可见状态。
// 使用 atomic.Bool 保证在 Wails 回调与 WindowEvent 之间的并发安全。
var windowVisible atomic.Bool

func init() {
	// 应用启动时窗口默认可见
	windowVisible.Store(true)
}

// startSysTray 初始化 Wails v3 原生系统托盘。
//
// 必须在 wailsApp.Run() 之前调用，Wails 会在 Run() 内部启动托盘。
// 与 getlantern/systray 不同，此方案的所有回调由 Wails 事件循环调度，
// 可以安全地调用任何窗口操作。
//
// 参数：
//   - wailsApp : Wails 应用实例
//   - win      : 主窗口（用于 Show/Hide/Focus）
//   - guiApp   : GUI 业务层（备用，当前暂不使用）
//   - onQuit   : 退出回调（先关闭 coreApp 再退出进程）
func startSysTray(
	wailsApp *application.App,
	win *application.WebviewWindow,
	_ *GuiApp,
	onQuit func(),
) {
	// ── 创建托盘实例 ──────────────────────────────────────────
	tray := wailsApp.SystemTray.New()

	// 设置图标与悬浮提示（tooltip 仅 Windows/Linux 有效）
	tray.SetIcon(trayIcon)
	tray.SetTooltip("Subs Check Pro - 订阅检测管理")

	// ── 构建右键菜单 ──────────────────────────────────────────
	menu := application.NewMenu()

	// 菜单项：显示主界面
	menu.Add("显示主界面").OnClick(func(_ *application.Context) {
		showWindow(win)
	})

	menu.AddSeparator()

	// 菜单项：退出
	menu.Add("退出 Subs Check Pro").OnClick(func(_ *application.Context) {
		slog.Info("用户通过托盘菜单退出")
		onQuit()
	})

	tray.SetMenu(menu)

	// ── 鼠标事件 ─────────────────────────────────────────────

	// 左键单击：切换显示/隐藏（Windows 常见托盘交互习惯）
	tray.OnClick(func() {
		if windowVisible.Load() {
			hideWindow(win)
		} else {
			showWindow(win)
		}
	})

	// 左键双击：强制显示窗口（防止误操作后找不到窗口）
	tray.OnDoubleClick(func() {
		showWindow(win)
	})

	// 右键单击：明确弹出菜单（部分平台默认右键才弹菜单）
	tray.OnRightClick(func() {
		tray.OpenMenu()
	})

	slog.Info("系统托盘初始化完成（Wails v3 原生）")
}

// showWindow 显示并聚焦窗口，同步可见状态标志。
func showWindow(win *application.WebviewWindow) {
	win.Show()
	win.Focus()
	windowVisible.Store(true)
	slog.Debug("窗口已显示")
}

// hideWindow 隐藏窗口，同步可见状态标志。
func hideWindow(win *application.WebviewWindow) {
	win.Hide()
	windowVisible.Store(false)
	slog.Debug("窗口已隐藏到托盘")
}

// NotifyHideToTray 在窗口最小化到托盘时调用，更新可见状态。
//
// Wails v3 alpha 阶段尚无内置气泡通知（Balloon Notification）API。
// 若需要气泡提示，可在此接入平台原生方案（如 Windows toast 或 go-toast）。
// 当前实现：仅记录日志 + 同步状态标志，保证 OnClick 切换逻辑的正确性。
func NotifyHideToTray() {
	windowVisible.Store(false)
	slog.Info("已最小化到系统托盘，单击托盘图标可恢复窗口")
}
