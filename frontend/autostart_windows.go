//go:build windows

// Package frontend: autostart_windows.go
//
// Windows 平台开机自启管理（Issue #6）。
// 使用 HKCU\Software\Microsoft\Windows\CurrentVersion\Run 注册表键。
package frontend

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const winRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const autostartAppName = "subs-check-pro-gui"

// isAutostartEnabled 检查 Windows 注册表中的开机自启状态。
func isAutostartEnabled() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return winAutostartEnabled(exe)
}

// setAutostart 在 Windows 注册表中设置或取消开机自启。
func setAutostart(enable bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	return winSetAutostart(enable, exe)
}

func winAutostartEnabled(exe string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, winRunKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	val, _, err := k.GetStringValue(autostartAppName)
	return err == nil && val != ""
}

func winSetAutostart(enable bool, exe string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, winRunKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表键失败: %w", err)
	}
	defer k.Close()

	if !enable {
		return k.DeleteValue(autostartAppName)
	}
	return k.SetStringValue(autostartAppName, exe)
}
