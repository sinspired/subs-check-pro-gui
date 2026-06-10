package updater

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type UpdateStatus struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
}

var (
	statusMu sync.RWMutex
	status   UpdateStatus
)

func GetUpdateStatus() UpdateStatus {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return status
}

func setUpdateStatus(v UpdateStatus) {
	statusMu.Lock()
	status = v
	statusMu.Unlock()
}

// CheckUpdateStatus 只做“检查是否有新版本”，不下载、不安装。
func CheckUpdateStatus(wailsApp *application.App) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rel, err := wailsApp.Updater.Check(ctx)
	if err != nil {
		slog.Debug("Updater check 失败", "error", err)
		return
	}

	if rel != nil {
		setUpdateStatus(UpdateStatus{
			Available: true,
			Version:   rel.Version,
		})
		return
	}

	setUpdateStatus(UpdateStatus{
		Available: false,
		Version:   "",
	})
}