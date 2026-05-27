//go:build !windows

// Package frontend: autostart.go
//
// 跨平台开机自启管理（Issue #6）—— 非 Windows 平台实现。
// 不依赖外部包，直接调用各平台原生机制：
//   - macOS：~/Library/LaunchAgents/<bundleID>.plist
//   - Linux：~/.config/autostart/<appName>.desktop
package frontend

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const autostartAppName = "subs-check-pro"
const autostartBundleID = "com.sinspired.subs-check-pro"

// isAutostartEnabled 检查当前平台的开机自启状态（非 Windows）。
func isAutostartEnabled() bool {
	switch runtime.GOOS {
	case "darwin":
		return macAutostartEnabled()
	default: // linux
		return linuxAutostartEnabled()
	}
}

// setAutostart 设置或取消开机自启（非 Windows）。
func setAutostart(enable bool) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	switch runtime.GOOS {
	case "darwin":
		return macSetAutostart(enable, exe)
	default:
		return linuxSetAutostart(enable, exe)
	}
}

// ── macOS ─────────────────────────────────────────────────────────────────────

func macLaunchAgentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", autostartBundleID+".plist")
}

func macAutostartEnabled() bool {
	_, err := os.Stat(macLaunchAgentPath())
	return err == nil
}

func macSetAutostart(enable bool, exe string) error {
	p := macLaunchAgentPath()
	if !enable {
		return os.Remove(p)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <false/>
</dict>
</plist>`, autostartBundleID, exe)
	return os.WriteFile(p, []byte(plist), 0o644)
}

// ── Linux ─────────────────────────────────────────────────────────────────────

func linuxDesktopPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", autostartAppName+".desktop")
}

func linuxAutostartEnabled() bool {
	_, err := os.Stat(linuxDesktopPath())
	return err == nil
}

func linuxSetAutostart(enable bool, exe string) error {
	p := linuxDesktopPath()
	if !enable {
		return os.Remove(p)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	desktop := strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Subs Check Pro",
		"Comment=订阅检测工具",
		"Exec=" + exe,
		"Hidden=false",
		"NoDisplay=false",
		"X-GNOME-Autostart-enabled=true",
	}, "\n") + "\n"
	return os.WriteFile(p, []byte(desktop), 0o644)
}
