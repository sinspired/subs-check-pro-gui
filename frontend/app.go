// Package frontend: app.go
// 定义 GuiApp 结构体，通过 Wails Bind 暴露给前端 JS。
package frontend

import (
	"context"
	"embed"
	"fmt"
	"net"
	"os"
	"strings"

	coreapp "github.com/sinspired/subs-check-pro/v2/app"
	"github.com/sinspired/subs-check-pro/v2/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed src
var assets embed.FS

// AppInfo 前端展示所需的应用运行信息。
type AppInfo struct {
	APIKey       string `json:"apiKey"`
	ListenPort   string `json:"listenPort"`
	SubStorePort string `json:"subStorePort"`
	// KeyIsRandom 为 true 表示 api-key 随机生成（重启后变更）
	KeyIsRandom bool `json:"keyIsRandom"`
	// IsFirstRun 为 true 表示本次是首次运行
	IsFirstRun bool `json:"isFirstRun"`
	// ConfigPath config.yaml 的实际路径
	ConfigPath string `json:"configPath"`
	// PortConflictHTTP 为 true 表示 HTTP 端口被占用
	PortConflictHTTP bool `json:"portConflictHTTP"`
	// PortConflictSubStore 为 true 表示 Sub-Store 端口被占用
	PortConflictSubStore bool `json:"portConflictSubStore"`
	// AutostartEnabled 当前平台开机自启状态
	AutostartEnabled bool `json:"autostartEnabled"`
}

// GuiApp Wails 绑定结构体。
type GuiApp struct {
	ctx        context.Context
	configPath string
	// backend 持有核心 App 实例，用于读取启动前端口预检结果等运行时状态。
	backend *coreapp.App
}

func (g *GuiApp) startup(ctx context.Context) {
	g.ctx = ctx
}

// GetAppInfo 返回应用运行信息（含端口冲突检测）。
// 端口冲突状态来自启动前的预检结果（Initialize() 在启动服务之前记录），
// 而非服务启动后的实时探测，避免将自身服务的端口误报为冲突。
func (g *GuiApp) GetAppInfo() AppInfo {
	port := strings.TrimPrefix(config.GlobalConfig.ListenPort, ":")
	if port == "" {
		port = "8199"
	}
	subStorePort := strings.TrimPrefix(config.GlobalConfig.SubStorePort, ":")

	var conflictHTTP, conflictSubStore bool
	if g.backend != nil {
		conflictHTTP = g.backend.PortConflictHTTP()
		conflictSubStore = g.backend.PortConflictSubStore()
	}

	return AppInfo{
		APIKey:               config.GlobalConfig.APIKey,
		ListenPort:           port,
		SubStorePort:         subStorePort,
		KeyIsRandom:          os.Getenv("GUI_KEY_IS_RANDOM") == "1",
		IsFirstRun:           os.Getenv("GUI_FIRST_RUN") == "1",
		ConfigPath:           g.configPath,
		PortConflictHTTP:     conflictHTTP,
		PortConflictSubStore: conflictSubStore,
		AutostartEnabled:     isAutostartEnabled(),
	}
}

// ResizeToMain 将窗口扩展至主界面尺寸。
func (g *GuiApp) ResizeToMain() {
	if g.ctx == nil {
		return
	}
	runtime.WindowSetSize(g.ctx, 1280, 860)
	runtime.WindowSetMinSize(g.ctx, 900, 600)
	runtime.WindowCenter(g.ctx)
}

// HideToTray 隐藏主窗口（最小化到系统托盘）。
func (g *GuiApp) HideToTray() {
	if g.ctx == nil {
		return
	}
	runtime.WindowHide(g.ctx)
}

// ShowWindow 恢复并激活主窗口。
func (g *GuiApp) ShowWindow() {
	if g.ctx == nil {
		return
	}
	runtime.WindowShow(g.ctx)
	runtime.WindowSetAlwaysOnTop(g.ctx, true)
	runtime.WindowSetAlwaysOnTop(g.ctx, false)
}

// GetEnterNonce 生成一次性 nonce，用于 /gui/enter 安全跳转（替代直接传 apiKey）。
// remember=true 时 /gui/enter 会将 key 写入 localStorage（跨会话）；
// remember=false 时只写 sessionStorage（本次标签页有效）。
func (g *GuiApp) GetEnterNonce(remember bool) string {
	return generateNonce(config.GlobalConfig.APIKey, remember)
}

// OpenConfigFile 弹出文件选择对话框，返回用户选择的 YAML 文件路径。
// 取消选择时返回空字符串。
func (g *GuiApp) OpenConfigFile() (string, error) {
	if g.ctx == nil {
		return "", fmt.Errorf("窗口未就绪")
	}
	path, err := runtime.OpenFileDialog(g.ctx, runtime.OpenDialogOptions{
		Title: "选择配置文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "YAML 配置文件 (*.yaml, *.yml)", Pattern: "*.yaml;*.yml"},
			{DisplayName: "所有文件", Pattern: "*"},
		},
	})
	return path, err
}

// ValidateConfigKey 验证给定 apiKey 是否与指定配置文件中的 api-key 匹配。
// 用于"选择配置文件"流程中的密码确认步骤。
// 若验证通过，生成并返回一次性 nonce（供 /gui/enter 使用）；否则返回错误。
func (g *GuiApp) ValidateConfigKey(enteredKey string, remember bool) (string, error) {
	actual := config.GlobalConfig.APIKey
	if actual == "" {
		return "", fmt.Errorf("配置文件未设置 api-key")
	}
	if strings.TrimSpace(enteredKey) != actual {
		return "", fmt.Errorf("密钥错误")
	}
	return generateNonce(actual, remember), nil
}

// SetPorts 更新 HTTP 端口和 Sub-Store 端口配置并检测冲突。
// 注意：修改后需重启服务器才能生效，此方法仅更新 GlobalConfig。
func (g *GuiApp) SetPorts(httpPort, subStorePort string) error {
	httpPort = strings.TrimPrefix(strings.TrimSpace(httpPort), ":")
	subStorePort = strings.TrimPrefix(strings.TrimSpace(subStorePort), ":")

	if httpPort == "" {
		return fmt.Errorf("HTTP 端口不能为空")
	}
	if isPortInUse(httpPort) {
		return fmt.Errorf("端口 %s 已被占用，请换一个", httpPort)
	}
	if subStorePort != "" && isPortInUse(subStorePort) {
		return fmt.Errorf("Sub-Store 端口 %s 已被占用，请换一个", subStorePort)
	}

	config.GlobalConfig.ListenPort = ":" + httpPort
	if subStorePort != "" {
		config.GlobalConfig.SubStorePort = ":" + subStorePort
	}
	return nil
}

// GetAutostartEnabled 返回当前平台的开机自启状态。
func (g *GuiApp) GetAutostartEnabled() bool {
	return isAutostartEnabled()
}

// SetAutostartEnabled 设置或取消开机自启。
func (g *GuiApp) SetAutostartEnabled(enable bool) error {
	return setAutostart(enable)
}

// isPortInUse 通过尝试监听来判断端口是否已被占用。
func isPortInUse(port string) bool {
	if port == "" {
		return false
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return true // 监听失败 = 端口占用
	}
	_ = ln.Close()
	return false
}
