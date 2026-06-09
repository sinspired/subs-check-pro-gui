// updater_debug.go
// 订阅所有 Wails updater 事件并追加写入文件日志。
//
// 设计目标：
//   - 即使点击「Restart & Apply」后进程退出，日志文件仍保留本次会话的完整记录
//   - 下次启动时继续追加，不清空旧记录，便于对比多次更新的行为差异
//   - helper 模式下也能记录哨兵变量，辅助诊断替换失败问题
//
// 日志位置：可执行文件同目录下的 updater-debug.log

package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"	
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

var (
	updaterLogFile *os.File
	updaterLogMu   sync.Mutex
)

// ensureUpdaterLogOpen 确保日志文件已打开（幂等，可多次调用）。
// 供 helper 模式下在 initUpdaterDebugLog 之前就写日志使用。
func ensureUpdaterLogOpen() {
	updaterLogMu.Lock()
	defer updaterLogMu.Unlock()
	if updaterLogFile != nil {
		return
	}
	f, err := os.OpenFile(updaterLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		slog.Warn("updater 日志文件创建失败", "error", err)
		return
	}
	updaterLogFile = f
}

// initUpdaterDebugLog 在 wailsApp 初始化完成、updater.Init 之后调用。
// 订阅全部 updater 事件，将关键信息追加写入日志文件。
func initUpdaterDebugLog(wailsApp *application.App) {
	ensureUpdaterLogOpen()

	exe, _ := os.Executable()
	writeUpdaterLog("=== 会话开始 pid=%d version=%s exe=%s ===",
		os.Getpid(), GuiVersion, exe)

	// ── 检查阶段 ──────────────────────────────────────────────────────────
	wailsApp.Event.On(updater.EventCheckStarted, func(_ *application.CustomEvent) {
		writeUpdaterLog("[check] 开始检查更新")
	})
	wailsApp.Event.On(updater.EventUpdateAvailable, func(e *application.CustomEvent) {
		if rel, ok := e.Data.(*updater.Release); ok {
			writeUpdaterLog("[check] 发现新版本 latest=%s current=%s", rel.Version, GuiVersion)
		} else {
			writeUpdaterLog("[check] 发现新版本（无版本信息）")
		}
	})
	wailsApp.Event.On(updater.EventNoUpdate, func(_ *application.CustomEvent) {
		writeUpdaterLog("[check] 已是最新版本")
	})

	// ── 下载阶段 ──────────────────────────────────────────────────────────
	wailsApp.Event.On(updater.EventDownloadStarted, func(e *application.CustomEvent) {
		if rel, ok := e.Data.(*updater.Release); ok {
			writeUpdaterLog("[download] 开始下载 version=%s", rel.Version)
		} else {
			writeUpdaterLog("[download] 开始下载")
		}
	})
	// 仅在 25% / 50% / 75% / 100% 四个节点写入，避免日志膨胀
	lastPct := -1
	wailsApp.Event.On(updater.EventDownloadProgress, func(e *application.CustomEvent) {
		p, ok := e.Data.(updater.Progress)
		if !ok || p.Total == 0 {
			return
		}
		pct := int(float64(p.Written) / float64(p.Total) * 100)
		milestone := pct / 25 * 25
		if milestone > lastPct && milestone > 0 {
			lastPct = milestone
			writeUpdaterLog("[download] %d%% (%d/%d bytes, %.1f KB/s)",
				milestone, p.Written, p.Total, float64(p.Rate)/1024)
		}
	})
	wailsApp.Event.On(updater.EventDownloadComplete, func(_ *application.CustomEvent) {
		writeUpdaterLog("[download] 下载完成")
	})

	// ── 校验 / 安装阶段 ───────────────────────────────────────────────────
	wailsApp.Event.On(updater.EventVerifying, func(_ *application.CustomEvent) {
		writeUpdaterLog("[verify] 开始校验完整性（SHA256SUMS）")
	})
	wailsApp.Event.On(updater.EventInstalling, func(_ *application.CustomEvent) {
		writeUpdaterLog("[install] 开始解压 / 暂存")
	})
	wailsApp.Event.On(updater.EventUpdateReady, func(_ *application.CustomEvent) {
		staged := wailsApp.Updater.DownloadedPath()
		writeUpdaterLog("[ready] 更新就绪")
		writeUpdaterLog("  state       = %s", wailsApp.Updater.State())
		writeUpdaterLog("  staged      = %s", staged)
		if staged != "" {
			if fi, err := os.Stat(staged); err != nil {
				writeUpdaterLog("  staged stat  = ERROR: %v", err)
			} else {
				writeUpdaterLog("  staged size  = %d bytes", fi.Size())
			}
		}
	})

	// ── 错误 ──────────────────────────────────────────────────────────────
	wailsApp.Event.On(updater.EventError, func(e *application.CustomEvent) {
		if info, ok := e.Data.(updater.ErrorInfo); ok {
			writeUpdaterLog("[ERROR] stage=%s message=%s", info.Stage, info.Message)
		} else {
			writeUpdaterLog("[ERROR] %+v", e.Data)
		}
	})

	// ── 用户操作 ─────────────────────────────────────────────────────────
	wailsApp.Event.On(updater.EventUserInstall, func(_ *application.CustomEvent) {
		writeUpdaterLog("[user] 点击「Install」（触发下载）")
	})
	wailsApp.Event.On(updater.EventUserRestart, func(_ *application.CustomEvent) {
		// 这是诊断「重启后未替换」的关键记录点
		staged := wailsApp.Updater.DownloadedPath()
		exe, _ := os.Executable()
		writeUpdaterLog("[user] 点击「Restart & Apply」")
		writeUpdaterLog("  state       = %s", wailsApp.Updater.State())
		writeUpdaterLog("  staged      = %s", staged)
		writeUpdaterLog("  executable  = %s", exe)
		if staged != "" {
			if fi, err := os.Stat(staged); err != nil {
				writeUpdaterLog("  staged stat  = ERROR: %v", err)
			} else {
				writeUpdaterLog("  staged size  = %d bytes", fi.Size())
			}
		}
		writeUpdaterLog("  >>> 父进程将退出，helper 负责替换后重启 <<<")
		flushUpdaterLog()
	})
	wailsApp.Event.On(updater.EventUserSkip, func(_ *application.CustomEvent) {
		writeUpdaterLog("[user] 点击「跳过此版本」")
	})
	wailsApp.Event.On(updater.EventUserRemind, func(_ *application.CustomEvent) {
		writeUpdaterLog("[user] 点击「稍后提醒」")
	})
	wailsApp.Event.On(updater.EventUserCancel, func(_ *application.CustomEvent) {
		writeUpdaterLog("[user] 关闭更新窗口（取消）")
	})

	logPath := updaterLogPath()
	slog.Info("updater 调试日志已启用", "path", logPath)
}

// writeUpdaterLog 线程安全地写入一行带时间戳的日志。
func writeUpdaterLog(format string, args ...any) {
	if updaterLogFile == nil {
		return
	}
	updaterLogMu.Lock()
	defer updaterLogMu.Unlock()
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05.000"), msg)
	_, _ = updaterLogFile.WriteString(line)
}

// flushUpdaterLog 强制将缓冲区写入磁盘（在进程即将退出前调用）。
func flushUpdaterLog() {
	if updaterLogFile == nil {
		return
	}
	updaterLogMu.Lock()
	defer updaterLogMu.Unlock()
	_ = updaterLogFile.Sync()
}

// updaterLogPath 返回日志文件路径：与可执行文件同目录；失败时退到临时目录。
func updaterLogPath() string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join(os.TempDir(), "subs-check-pro-gui-updater.log")
	}
	return filepath.Join(filepath.Dir(exe), "updater-debug.log")
}