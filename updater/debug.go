package updater

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
	logFile *os.File
	logMu   sync.Mutex
)

func ensureLogOpen() {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		return
	}
	f, err := os.OpenFile(logPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		slog.Warn("updater 日志文件创建失败", "error", err)
		return
	}
	logFile = f
}

// InitDebugLog 在 wailsApp 初始化完成、updater.Init 之后调用，
// 订阅全部 updater 事件并写入 updater-debug.log（与可执行文件同目录）。
// guiVersion 由调用方（main 包）通过 ldflags 注入后传入。
func InitDebugLog(wailsApp *application.App, guiVersion string) {
	ensureLogOpen()

	exe, _ := os.Executable()
	writeLog("=== 会话开始 pid=%d version=%s exe=%s ===", os.Getpid(), guiVersion, exe)

	wailsApp.Event.On(updater.EventCheckStarted, func(_ *application.CustomEvent) {
		writeLog("[check] 开始检查更新")
	})
	wailsApp.Event.On(updater.EventUpdateAvailable, func(e *application.CustomEvent) {
		if rel, ok := e.Data.(*updater.Release); ok {
			writeLog("[check] 发现新版本 latest=%s current=%s", rel.Version, guiVersion)
		} else {
			writeLog("[check] 发现新版本（无版本信息）")
		}
	})
	wailsApp.Event.On(updater.EventNoUpdate, func(_ *application.CustomEvent) {
		writeLog("[check] 已是最新版本")
	})

	wailsApp.Event.On(updater.EventDownloadStarted, func(e *application.CustomEvent) {
		if rel, ok := e.Data.(*updater.Release); ok {
			writeLog("[download] 开始下载 version=%s", rel.Version)
		} else {
			writeLog("[download] 开始下载")
		}
	})
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
			writeLog("[download] %d%% (%d/%d bytes, %.1f KB/s)", milestone, p.Written, p.Total, float64(p.Rate)/1024)
		}
	})
	wailsApp.Event.On(updater.EventDownloadComplete, func(_ *application.CustomEvent) {
		writeLog("[download] 下载完成")
	})

	wailsApp.Event.On(updater.EventVerifying, func(_ *application.CustomEvent) {
		writeLog("[verify] 开始校验完整性（SHA256SUMS）")
	})
	wailsApp.Event.On(updater.EventInstalling, func(_ *application.CustomEvent) {
		writeLog("[install] 开始解压 / 暂存")
	})
	wailsApp.Event.On(updater.EventUpdateReady, func(_ *application.CustomEvent) {
		staged := wailsApp.Updater.DownloadedPath()
		writeLog("[ready] 更新就绪")
		writeLog("  state  = %s", wailsApp.Updater.State())
		writeLog("  staged = %s", staged)
		logStagedSize(staged)
	})

	wailsApp.Event.On(updater.EventError, func(e *application.CustomEvent) {
		if info, ok := e.Data.(updater.ErrorInfo); ok {
			writeLog("[ERROR] stage=%s message=%s", info.Stage, info.Message)
		} else {
			writeLog("[ERROR] %+v", e.Data)
		}
	})

	wailsApp.Event.On(updater.EventUserInstall, func(_ *application.CustomEvent) {
		writeLog("[user] 点击「Install」（触发下载）")
	})
	wailsApp.Event.On(updater.EventUserRestart, func(_ *application.CustomEvent) {
		staged := wailsApp.Updater.DownloadedPath()
		exe, _ := os.Executable()
		writeLog("[user] 点击「Restart & Apply」")
		writeLog("  state      = %s", wailsApp.Updater.State())
		writeLog("  staged     = %s", staged)
		writeLog("  executable = %s", exe)
		logStagedSize(staged)
		writeLog("  >>> 父进程将退出，helper 负责替换后重启 <<<")
		flushLog()
	})
	wailsApp.Event.On(updater.EventUserSkip, func(_ *application.CustomEvent) {
		writeLog("[user] 点击「跳过此版本」")
	})
	wailsApp.Event.On(updater.EventUserRemind, func(_ *application.CustomEvent) {
		writeLog("[user] 点击「稍后提醒」")
	})
	wailsApp.Event.On(updater.EventUserCancel, func(_ *application.CustomEvent) {
		writeLog("[user] 关闭更新窗口（取消）")
	})

	slog.Debug("updater 调试日志已启用", "path", logPath())
}

func logStagedSize(staged string) {
	if staged == "" {
		return
	}
	if fi, err := os.Stat(staged); err != nil {
		writeLog("  staged stat = ERROR: %v", err)
	} else {
		writeLog("  staged size = %d bytes", fi.Size())
	}
}

func writeLog(format string, args ...any) {
	if logFile == nil {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	line := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05.000"), fmt.Sprintf(format, args...))
	_, _ = logFile.WriteString(line)
}

func flushLog() {
	if logFile == nil {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	_ = logFile.Sync()
}

func logPath() string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join(os.TempDir(), "subs-check-pro-gui-updater.log")
	}
	return filepath.Join(filepath.Dir(exe), "updater-debug.log")
}
