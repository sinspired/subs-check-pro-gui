//go:build !windows && !darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Linux XDG autostart 实现

func queryAutoStart() (bool, error) {
	desktopFile, err := autostartDesktopPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(desktopFile)
	return err == nil, nil
}

func applyAutoStart(enable bool) error {
	desktopFile, err := autostartDesktopPath()
	if err != nil {
		return err
	}
	if !enable {
		_ = os.Remove(desktopFile)
		return nil
	}
	execPath, err := getAutostartExecPath()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Subs Check Pro
Exec=%s
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
`, execPath)
	dir := filepath.Dir(desktopFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(desktopFile, []byte(content), 0644)
}

func autostartDesktopPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" || !strings.HasPrefix(xdgConfig, "/") {
		xdgConfig = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfig, "autostart", "subs-check-pro.desktop"), nil
}
