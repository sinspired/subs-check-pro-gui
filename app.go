package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	coreapp "github.com/sinspired/subs-check-pro/v2/app"
	"github.com/sinspired/subs-check-pro/v2/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// GuiApp Wails 绑定结构体。
type GuiApp struct {
	configPath string
	backend    *coreapp.App
	// window 持有登录窗口引用，由 main.go 在创建窗口后注入。
	// 用于 ResizeToMain() 调整窗口尺寸，避免依赖 GetWindowByName（alpha 不可用）。
	window *application.WebviewWindow
}

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

// GetAppInfo 返回应用运行信息（含端口冲突检测）。
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
	}
}

// GetEnterNonce 生成一次性 nonce，用于 /gui/enter 安全跳转。
func (g *GuiApp) GetEnterNonce(remember bool) string {
	return generateNonce(config.GlobalConfig.APIKey, remember)
}

// ValidateConfigKey 验证密钥，通过后返回一次性 nonce。
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

// SetPorts 更新端口配置并检测冲突。
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

// OpenConfigFile 打开系统文件选择对话框，返回用户选择的配置文件路径。
// 用户取消时返回空字符串。
func (g *GuiApp) OpenConfigFile() (string, error) {
	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("应用实例不可用")
	}

	result, err := app.Dialog.OpenFile().
		SetTitle("选择配置文件").
		AddFilter("YAML 配置文件", "*.yaml;*.yml").
		PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("打开文件对话框失败: %w", err)
	}
	return result, nil
}

// ResizeToMain 将登录窗口扩展为主界面尺寸。
func (g *GuiApp) ResizeToMain() {
	if g.window == nil {
		return
	}
	g.window.SetSize(1024, 768)
	g.window.Center()
}

// isPortInUse 通过尝试监听来判断端口是否已被占用。
func isPortInUse(port string) bool {
	if port == "" {
		return false
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}