// Package updater: window.go
// 嵌入自定义更新窗口 HTML，与项目侘寂风格保持一致。
package updater

import _ "embed"

// CustomWindowHTML 是与主窗口风格一致的自定义 Wails 更新器窗口 HTML。
// 由 main.go 中的 updater.BuiltinWindow{HTML: ...} 引用。
//
//go:embed updater_window.html
var CustomWindowHTML string
