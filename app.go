package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	coreapp "github.com/sinspired/subs-check-pro/v2/app"
	"github.com/sinspired/subs-check-pro/v2/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// GuiApp Wails 绑定结构体。
type GuiApp struct {
	configPath string
	backend    *coreapp.App
	// window 持有主窗口引用，由 main.go 在创建窗口后注入。
	window *application.WebviewWindow

	// pendingInit 为 true 时表示端口预检发现冲突，Initialize() 尚未调用。
	// 前端通过 GetAppInfo().portConflictHTTP/portConflictSubStore 感知，
	// 用户修正端口后调用 CompleteInit() 完成初始化。
	pendingInit     bool
	preConflictHTTP bool
	preConflictSub  bool
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
	// PendingInit 为 true 表示后端尚未初始化，需要前端先解决端口冲突
	PendingInit bool `json:"pendingInit"`
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
	if g.pendingInit {
		// 预检阶段：使用预检结果
		conflictHTTP = g.preConflictHTTP
		conflictSubStore = g.preConflictSub
	} else if g.backend != nil {
		// 正常运行阶段：使用后端运行时结果
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
		PendingInit:          g.pendingInit,
	}
}

// CompleteInit 在用户修正端口冲突后，由前端调用，完成后端初始化。
// 仅当 pendingInit==true 时有效；初始化完成后自动清除 pending 状态。
func (g *GuiApp) CompleteInit() error {
	if !g.pendingInit {
		return nil // 已初始化，幂等返回
	}

	if g.backend == nil {
		return fmt.Errorf("内部错误：backend 未设置")
	}

	if err := g.backend.Initialize(); err != nil {
		return fmt.Errorf("初始化后端失败: %w", err)
	}

	if err := g.backend.EnsureRouter(); err != nil {
		return fmt.Errorf("初始化 HTTP 路由失败: %w", err)
	}

	registerGuiAutoLogin(g.backend.GetRouter())

	go g.backend.Run()

	// 更新 configPath（Initialize 可能已解析出真实路径）
	g.configPath = g.backend.GetConfigPath()
	g.pendingInit = false
	g.preConflictHTTP = false
	g.preConflictSub = false

	sendOSNotification("Subs Check Pro", "服务已成功启动")
	return nil
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

// ValidatePort 实时验证单个端口号合法性，供前端输入框即时校验调用。
// 返回空字符串表示合法；否则返回可直接展示的错误描述。
func (g *GuiApp) ValidatePort(port string) string {
	if err := validatePort(port); err != nil {
		return err.Error()
	}
	return ""
}

// SetPorts 更新端口配置。
// 先做合法性校验（数字、范围 1024-65535、两端口不重复），再做占用检测。
func (g *GuiApp) SetPorts(httpPort, subStorePort string) error {
	httpPort = normalizePort(httpPort)
	subStorePort = normalizePort(subStorePort)

	// 合法性校验
	if err := validatePort(httpPort); err != nil {
		return fmt.Errorf("HTTP 端口无效: %w", err)
	}
	if subStorePort != "" {
		if err := validatePort(subStorePort); err != nil {
			return fmt.Errorf("Sub-Store 端口无效: %w", err)
		}
		if httpPort == subStorePort {
			return fmt.Errorf("HTTP 端口与 Sub-Store 端口不能相同（均为 %s）", httpPort)
		}
	}

	// 端口占用检测
	if isPortInUse(httpPort) {
		return fmt.Errorf("HTTP 端口 %s 已被占用，请换一个", httpPort)
	}
	if subStorePort != "" && isPortInUse(subStorePort) {
		return fmt.Errorf("Sub-Store 端口 %s 已被占用，请换一个", subStorePort)
	}

	// 写入配置
	config.GlobalConfig.ListenPort = ":" + httpPort
	if subStorePort != "" {
		config.GlobalConfig.SubStorePort = ":" + subStorePort
	}

	// 更新预检冲突状态（用户已选择了可用端口）
	g.preConflictHTTP = false
	g.preConflictSub = false

	return nil
}

// ResizeToMain 将登录小窗切换为管理界面大窗。
func (g *GuiApp) ResizeToMain() {
	if g.window == nil {
		return
	}
	g.window.SetSize(1024, 768)
	g.window.Center()
	g.window.Focus()
	windowVisible.Store(true)
}

// ShowWindow 供前端主动调用，显示并聚焦主窗口。
func (g *GuiApp) ShowWindow() {
	if g.window == nil {
		return
	}
	g.window.Show()
	g.window.Focus()
	windowVisible.Store(true)
}

// HideToTray 供前端"关闭按钮对话框"选择最小化时调用。
func (g *GuiApp) HideToTray() {
	if g.window == nil {
		return
	}
	hideWindow(g.window)
	sendOSNotification("Subs Check Pro", "已最小化到系统托盘\n单击托盘图标可恢复窗口")
}

// QuitApp 供前端"关闭按钮对话框"选择退出时调用。
// 这里只发起退出请求；真正退出后的“已退出”通知由 OnShutdown 统一发送。
func (g *GuiApp) QuitApp() {
	sendOSNotification("Subs Check Pro", "正在退出…")
	app := application.Get()
	if app != nil {
		app.Quit()
	}
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

// 端口校验辅助
// normalizePort 去除前缀冒号和空格，统一为纯数字字符串。
func normalizePort(port string) string {
	return strings.TrimPrefix(strings.TrimSpace(port), ":")
}

// validatePort 校验端口号合法性。
//
// 规则：
//   - 不能为空
//   - 必须是纯数字
//   - 范围 1024-65535（1-1023 为系统保留端口，需 root 权限）
func validatePort(port string) error {
	port = normalizePort(port)

	if port == "" {
		return fmt.Errorf("端口不能为空")
	}

	p, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("端口必须为数字，当前值: %q", port)
	}

	switch {
	case p < 1:
		return fmt.Errorf("端口不能小于 1")
	case p < 1024:
		// 系统保留端口，普通进程无权限绑定（Windows 需管理员，Unix 需 root）
		return fmt.Errorf("端口 %d 是系统保留端口（1-1023），请使用 1024-65535 范围内的端口", p)
	case p > 65535:
		return fmt.Errorf("端口不能大于 65535，当前值: %d", p)
	}

	return nil
}

// isPortInUse 通过尝试绑定来判断端口是否已被占用。
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
