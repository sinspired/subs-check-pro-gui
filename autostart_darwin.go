//go:build darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const plistName = "com.sinspired.subs-check-pro.plist"

func queryAutoStart() (bool, error) {
	p, err := launchAgentPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	return err == nil, nil
}

func applyAutoStart(enable bool) error {
	p, err := launchAgentPath()
	if err != nil {
		return err
	}
	if !enable {
		_ = os.Remove(p)
		return nil
	}
	execPath, err := getAutostartExecPath()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
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
</dict>
</plist>
`, plistName, execPath)
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(plist), 0644)
}

func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistName), nil
}


