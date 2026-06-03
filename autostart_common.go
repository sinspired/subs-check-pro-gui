package main

import "os"

// getAutostartExecPath 返回当前可执行文件路径，用于写入自启配置。
func getAutostartExecPath() (string, error) {
	return os.Executable()
}
