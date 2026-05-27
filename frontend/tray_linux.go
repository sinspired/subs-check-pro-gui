//go:build linux

// Package frontend: tray_linux.go
//
// Linux 行为：
//   - 关闭窗口 → 正常退出（OnBeforeClose 返回 false）
//   - 不弹「已最小化到托盘」提示
package frontend

import "context"

// platformHasTray Linux 不支持系统托盘。
const platformHasTray = false

// startSysTray Linux 空实现，直接返回。
func startSysTray(_ context.Context, _ *GuiApp, _ func()) {}

// NotifyHideToTray Linux 空实现，不弹窗。
func NotifyHideToTray(_ context.Context) {}