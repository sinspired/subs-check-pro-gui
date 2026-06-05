// Package main: tray.go
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sinspired/subs-check-pro/v2/app"
	"github.com/sinspired/subs-check-pro/v2/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// trayIcon 嵌入托盘图标文件（ICO 格式，Windows/Linux 通用）。
// macOS 建议改用透明背景 PNG 并调用 SetTemplateIcon。
//
//go:embed frontend/public/logo.png
var trayIcon []byte

//go:embed frontend/public/logo_32x32.png
var logo32 []byte

// windowVisible 跟踪当前窗口可见状态。
var windowVisible atomic.Bool

func init() {
	windowVisible.Store(true)
}

// 退出状态机
var (
	gracefulQuitPending atomic.Bool // 首次“结束检测后退出”已触发
	gracefulQuitOnce    sync.Once   // 保证后台等待 goroutine 只启动一次
)

// startSysTray 初始化 Wails v3 原生系统托盘。
// 参数：
//   - wailsApp : Wails 应用实例
//   - guiApp   : GUI 业务层（提供 showActiveWindow / hideActiveWindow）
//   - coreApp  : 核心业务实例（用于发送终止检测信号）
//   - notifier : 通知服务
//   - onQuit   : 退出回调（先关闭 coreApp 再退出进程）
func startSysTray(
	wailsApp *application.App,
	guiApp *GuiApp,
	coreApp *app.App,
	appInitOK bool,
	onQuit func(),
) {
	// 创建托盘实例
	tray := wailsApp.SystemTray.New()

	// 设置图标与悬浮提示（tooltip 仅 Windows/Linux 有效）
	tray.SetIcon(trayIcon)
	tray.SetTooltip(formatSysTrayTooltip(coreApp, appInitOK)) // 传入参数以获取状态

	// 提供更新 Tooltip 的回调，传递给 buildTrayMenu 以便定时同步刷新
	updateTooltip := func() {
		tray.SetTooltip(formatSysTrayTooltip(coreApp, appInitOK))
	}

	// 构建右键菜单
	menu := buildTrayMenu(wailsApp, guiApp, coreApp, appInitOK, onQuit, updateTooltip)
	tray.SetMenu(menu)

	// 左键单击：切换当前活跃窗口的显示/隐藏
	tray.OnClick(func() {
		if windowVisible.Load() {
			guiApp.hideActiveWindow()
		} else {
			guiApp.showActiveWindow()
		}
	})

	// 左键双击：强制显示当前活跃窗口
	tray.OnDoubleClick(func() {
		guiApp.showActiveWindow()
	})

	// 右键单击：弹出菜单
	tray.OnRightClick(func() {
		tray.OpenMenu()
	})

	slog.Debug("系统托盘初始化完成（Wails v3 原生）")
}

func buildTrayMenu(
	wailsApp *application.App,
	guiApp *GuiApp,
	coreApp *app.App,
	appInitOK bool,
	onQuit func(),
	updateTooltip func(),
) *application.Menu {
	menu := wailsApp.NewMenu()

	menu.Add("Subs Check Pro 桌面端").SetBitmap(logo32).SetEnabled(false)

	// 状态显示菜单项
	statusItem := menu.Add("...")
	statusItem.SetEnabled(false) // 设为不可点击，纯展示用途

	menu.AddSeparator()

	menu.Add("显示主界面").OnClick(func(_ *application.Context) {
		guiApp.showActiveWindow()
	})

	menu.Add("隐藏主界面").OnClick(func(_ *application.Context) {
		guiApp.hideActiveWindow()
		sendOSNotification("Subs Check Pro", "已最小化到系统托盘\n单击图标可恢复窗口")
	})

	menu.AddSeparator()

	menu.Add("返回登录窗口").OnClick(func(_ *application.Context) {
		guiApp.BackToLogin()
		sendOSNotification("Subs Check Pro", "已返回登录窗口")
	})

	menu.AddSeparator()

	triggerCheckMenu := menu.Add("开始检测")
	triggerCheckMenu.OnClick(func(_ *application.Context) {
		coreApp.TriggerCheck()
		sendOSNotification("Subs Check Pro", "已触发检测任务\n可在系统托盘、管理界面查看检测进度")
		triggerCheckMenu.SetEnabled(false)
	})

	stopCheckMenu := menu.Add("停止检测")
	stopCheckMenu.OnClick(func(_ *application.Context) {
		if err := callBackendForceClose(); err != nil {
			slog.Warn("检测任务停止失败", "error", err)
		} else {
			sendOSNotification("Subs Check Pro", "已发送停止检测信号\n正在等待结果收集完成")
			slog.Debug("托盘：已发送停止检测信号")
		}
	})

	menu.AddSeparator()

	stopCheckAndExitMenu := menu.Add("结束检测并退出")
	stopCheckAndExitMenu.OnClick(func(_ *application.Context) {
		if gracefulQuitPending.CompareAndSwap(false, true) {
			sendOSNotification("Subs Check Pro", "正在等待检测完成后退出\n再次点击将立即强制退出")
			slog.Debug("托盘：已发送停止检测信号，等待检测完成后退出")

			gracefulQuitOnce.Do(func() {
				go func() {
					if err := callBackendForceClose(); err != nil {
						slog.Warn("发送 force-close 失败", "error", err)
					}

					waitForBackendIdle(5 * time.Minute)

					slog.Info("后端检测已完成，开始优雅退出")
					sendOSNotification("Subs Check Pro", "检测已完成，正在退出…")
					onQuit()
				}()
			})
		} else {
			slog.Warn("GUI：用户二次确认，立即退出")
			sendOSNotification("Subs Check Pro", "正在强制退出…")
			onQuit()
		}
	})

	menu.AddSeparator()
	// ── 开机自启（Checkbox 菜单项，自动显示对勾）────────────────────
	autostartItem := menu.Add("开机自启")
	autostartItem.SetChecked(false) // 默认未勾选，异步更新

	// 将菜单项引用存入 guiApp，供前端调用 SetAutoStart 后反向同步 checkbox
	guiApp.autostartMenuItem = autostartItem

	autostartItem.OnClick(func(_ *application.Context) {
		enabled, err := guiApp.GetAutoStartEnabled()
		if err != nil {
			slog.Warn("读取开机自启状态失败", "error", err)
			sendOSNotification("Subs Check Pro", "读取开机自启状态失败")
			return
		}
		next := !enabled
		if err := guiApp.SetAutoStartEnabled(next); err != nil {
			slog.Warn("设置开机自启失败", "error", err)
			sendOSNotification("Subs Check Pro", "设置开机自启失败："+err.Error())
			return
		}
		autostartItem.SetChecked(next)
		// 向前端登录窗口发射事件，同步开机自启按钮状态
		if guiApp.loginWin != nil {
			guiApp.loginWin.EmitEvent("autostart:changed", next)
		}
		if next {
			sendOSNotification("Subs Check Pro", "已开启开机自启")
		} else {
			sendOSNotification("Subs Check Pro", "已关闭开机自启")
		}
	})

	menu.AddSeparator()

	// About menu
	menu.Add("关于").OnClick(func(_ *application.Context) {
		guiApp.OpenAboutWindow()
	})

	menu.Add("退出").OnClick(func(_ *application.Context) {
		slog.Info("GUI：强制退出")
		sendOSNotification("Subs Check Pro", "正在退出…")
		onQuit()
	})

	// 启动后台协程，定时（如 1.5 秒）更新状态文本
	go func() {
		enabled, err := guiApp.GetAutoStartEnabled()
		if err != nil {
			slog.Warn("初始化开机自启 checkbox 失败", "error", err)
			return
		}
		autostartItem.SetChecked(enabled)

		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			// 同步刷新托盘的 Tooltip 悬浮提示
			if updateTooltip != nil {
				updateTooltip()
			}

			if !appInitOK || coreApp == nil {
				statusItem.SetLabel("后端未启动")
				continue
			}

			// 1. 如果正在检测中
			if coreApp.IsChecking() {
				stopCheckMenu.SetEnabled(true)
				stopCheckAndExitMenu.SetEnabled(true)
				statusItem.SetHidden(false)

				// 生成进度信息
				progressStr := renderProgressString(coreApp)

				// 在托盘显示检测进度
				statusItem.SetLabel(progressStr)

				continue
			}

			// 2. 如果检测已完成（或空闲中）
			lastResult := coreApp.GetLastCheckResult()
			if lastResult != "" {
				statusItem.SetHidden(false)
				statusItem.SetLabel(lastResult)
			} else {
				statusItem.SetHidden(true)
			}

			triggerCheckMenu.SetEnabled(true)
			stopCheckMenu.SetEnabled(false)
			stopCheckAndExitMenu.SetEnabled(false)
		}
	}()

	return menu
}

// 后端 HTTP API 辅助函数

func backendBase() string {
	port := strings.TrimPrefix(config.GlobalConfig.ListenPort, ":")
	if port == "" {
		port = "8199"
	}
	return "http://127.0.0.1:" + port
}

func callBackendForceClose() error {
	url := backendBase() + "/api/force-close"
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", config.GlobalConfig.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func isBackendChecking() bool {
	url := backendBase() + "/api/status"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return true
	}
	req.Header.Set("X-API-Key", config.GlobalConfig.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return true
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return true
	}

	var status struct {
		Checking bool `json:"checking"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		return true
	}
	return status.Checking
}

func waitForBackendIdle(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isBackendChecking() {
			return
		}
		time.Sleep(2 * time.Second)
	}
	slog.Warn("等待后端空闲超时，继续退出流程")
}

// showWindow 窗口显示/隐藏
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
func NotifyHideToTray() {
	windowVisible.Store(false)
	sendOSNotification("Subs Check Pro", "已最小化到系统托盘\n单击托盘图标可恢复窗口")
	slog.Debug("已最小化到系统托盘，单击托盘图标可恢复窗口")
}

// renderProgressString 根据当前状态生成进度字符串
func renderProgressString(coreApp *app.App) string {
	var percent float64
	state := coreApp.GetCurrentState()
	stepName := state.StepName

	if state.ProxyCount == 0 {
		if stepName == "保存中" {
			percent = 100.0
		} else {
			percent = 0.0
		}
	} else {
		percent = float64(state.Progress) / float64(state.ProxyCount) * 100.0
	}

	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	if stepName == "" {
		stepName = "进度"
	}

	return fmt.Sprintf("%s %.1f%% %s", stepName, percent, state.ETASuffix)
}

// formatSysTrayTooltip 构建托盘悬浮提示文本，包含应用名称、当前监听端口以及检测进度。
func formatSysTrayTooltip(coreApp *app.App, appInitOK bool) string {
	base := "Subs Check Pro GUI" + " - 端口 " + strings.TrimPrefix(config.GlobalConfig.ListenPort, ":")

	if !appInitOK || coreApp == nil {
		return base + "\n后端未启动"
	}

	if coreApp.IsChecking() {
		progressStr := renderProgressString(coreApp)

		// 检测中：下一行显示当前进度格式化字符串
		return base + "\n" + progressStr
	}

	lastResult := coreApp.GetLastCheckResult()
	if lastResult != "" {
		return base + "\n" + lastResult + "\n空闲 √"
	} else {
		return base + "\n空闲 √"
	}
}
