// Package frontend: nonce.go
//
// 一次性 nonce 管理，用于 GUI 自动登录安全增强（Issue #5）。
//
// 流程：
//   1. Wails 前端调用 GetEnterNonce(remember) 获取一次性 nonce
//   2. 前端导航到 /gui/enter?n=<nonce>（而非 ?t=<apiKey>）
//   3. /gui/enter 通过 consumeNonce 换取 apiKey，nonce 立即作废
//
// 安全特性：
//   - nonce 随机生成，32 字节，不含 apiKey 信息
//   - 单次有效，使用后立即删除
//   - 30 秒过期自动清理
//   - apiKey 不再出现在浏览器 URL、历史记录或代理日志中
package frontend

import (
	"sync"
	"time"

	"github.com/sinspired/subs-check-pro/v2/utils"
)

const nonceTTL = 30 * time.Second

type nonceEntry struct {
	apiKey    string
	remember  bool
	expiresAt time.Time
}

var (
	nonceMu     sync.Mutex
	guiNonces   = make(map[string]nonceEntry)
	nonceOnce   sync.Once // 确保后台清理只启动一次
)

// generateNonce 生成一次性 nonce，绑定 apiKey 和 remember 标志，30 秒后自动过期。
func generateNonce(apiKey string, remember bool) string {
	nonceOnce.Do(startNonceCleanup)

	nonce := utils.GenerateRandomString(32)
	nonceMu.Lock()
	guiNonces[nonce] = nonceEntry{
		apiKey:    apiKey,
		remember:  remember,
		expiresAt: time.Now().Add(nonceTTL),
	}
	nonceMu.Unlock()
	return nonce
}

// consumeNonce 查找并消费 nonce，返回绑定的 apiKey 与 remember 标志。
// nonce 使用一次后立即删除；过期 nonce 同样返回 false。
func consumeNonce(nonce string) (apiKey string, remember bool, ok bool) {
	nonceMu.Lock()
	defer nonceMu.Unlock()

	entry, exists := guiNonces[nonce]
	if !exists {
		return "", false, false
	}
	delete(guiNonces, nonce) // 单次有效，立即删除
	if time.Now().After(entry.expiresAt) {
		return "", false, false
	}
	return entry.apiKey, entry.remember, true
}

// startNonceCleanup 启动后台协程，每 10 秒清理过期 nonce，防止内存泄漏。
func startNonceCleanup() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			nonceMu.Lock()
			for k, v := range guiNonces {
				if now.After(v.expiresAt) {
					delete(guiNonces, k)
				}
			}
			nonceMu.Unlock()
		}
	}()
}
