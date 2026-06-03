//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const regRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const regAppName = "SubsCheckPro"

func queryAutoStart() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, regRunKey, registry.QUERY_VALUE)
	if err != nil {
		return false, nil
	}
	defer k.Close()
	_, _, err = k.GetStringValue(regAppName)
	return err == nil, nil
}

func applyAutoStart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regRunKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %w", err)
	}
	defer k.Close()
	if !enable {
		_ = k.DeleteValue(regAppName)
		return nil
	}
	execPath, err := getAutostartExecPath()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	return k.SetStringValue(regAppName, `"`+execPath+`"`)
}


