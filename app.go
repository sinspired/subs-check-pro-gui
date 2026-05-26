package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	coreapp "github.com/sinspired/subs-check-pro/v2/app"
	"github.com/sinspired/subs-check-pro/v2/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// GuiApp Wails 绑定结构体。
type GuiApp struct {
	configPath string
	backend    *coreapp.App
	// window 持有主窗口引用（loginWin 的别名），兼容托盘等旧引用。
	// 由 main.go 在创建窗口后注入。
	window *application.WebviewWindow

	// loginWin 登录小窗（加载 Wails 前端资产）。
	// 由 main.go 在创建窗口后注入。
	loginWin *application.WebviewWindow

	// webUIWin WebUI 大窗（加载外部 Gin 服务，初始隐藏）。
	// 由 main.go 在创建窗口后注入。
	webUIWin *application.WebviewWindow

	// pendingInit 为 true 时表示端口预检发现冲突，Initialize() 尚未调用。
	pendingInit     bool
	preConflictHTTP bool
	preConflictSub  bool

	// inWebUI 为 true 表示窗口已切换到外部 WebUI 页面。
	// 此时 Wails JS runtime 不可用，关闭事件须走 Go 原生对话框。
	inWebUI atomic.Bool
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

// EnterWebUI 由前端调用：导航 WebUI 窗口、显示它、隐藏登录窗口。
// 完全无定时器，无闪烁。
func (g *GuiApp) EnterWebUI(enterURL string) {
	if g.webUIWin == nil || g.loginWin == nil {
		return
	}
	g.inWebUI.Store(true)
	// Wails v3 中导航到指定 URL 的正确方法是 SetURL，Navigate 已不存在
	g.webUIWin.SetURL(enterURL)
	g.webUIWin.Show()
	g.webUIWin.Center()
	g.webUIWin.Focus()
	g.loginWin.Hide()
}

// BackToLogin 从 WebUI 返回登录窗口（可选功能，供托盘菜单使用）
func (g *GuiApp) BackToLogin() {
	if g.loginWin == nil {
		return
	}
	g.inWebUI.Store(false)
	g.loginWin.Show()
	g.loginWin.Center()
	g.loginWin.Focus()
	g.webUIWin.Hide()
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
		conflictHTTP = g.preConflictHTTP
		conflictSubStore = g.preConflictSub
	} else if g.backend != nil {
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
func (g *GuiApp) CompleteInit() error {
	if !g.pendingInit {
		return nil
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

// ValidatePort 实时验证单个端口号合法性。
func (g *GuiApp) ValidatePort(port string) string {
	if err := validatePort(port); err != nil {
		return err.Error()
	}
	return ""
}

// SetPorts 更新端口配置。
func (g *GuiApp) SetPorts(httpPort, subStorePort string) error {
	httpPort = normalizePort(httpPort)
	subStorePort = normalizePort(subStorePort)

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

	if isPortInUse(httpPort) {
		return fmt.Errorf("HTTP 端口 %s 已被占用，请换一个", httpPort)
	}
	if subStorePort != "" && isPortInUse(subStorePort) {
		return fmt.Errorf("Sub-Store 端口 %s 已被占用，请换一个", subStorePort)
	}

	config.GlobalConfig.ListenPort = ":" + httpPort
	if subStorePort != "" {
		config.GlobalConfig.SubStorePort = ":" + subStorePort
	}

	g.preConflictHTTP = false
	g.preConflictSub = false

	return nil
}

// ResizeToMain 将登录小窗无闪烁地切换为管理界面大窗。
//
// 实现策略（Wails v3 原生方式）：
//  1. 标记进入 WebUI 模式（关闭按钮改走 Go 原生对话框）
//  2. 立即隐藏窗口（用户看不到后续的尺寸/导航变化）
//  3. 调整窗口大小并居中
//  4. 启动定时器，在外部页面加载完成后再显示窗口
//
// 前端在此函数返回后立即执行 window.location.replace()，
// 定时器在导航和页面渲染完成后触发 Show()，实现无感切换。
func (g *GuiApp) ResizeToMain() {
	if g.window == nil {
		return
	}

	// 标记已进入 WebUI；关闭钩子将改用 Go 原生对话框
	g.inWebUI.Store(true)

	// 隐藏窗口——从此刻起用户看不到任何 resize / 页面切换的闪烁
	g.window.Hide()
	g.window.SetSize(1024, 768)
	g.window.Center()

	// 600 ms 后显示：给 window.location.replace 和本地页面加载足够时间
	// localhost 页面通常 <100 ms 加载完毕，600 ms 有充足余量
	time.AfterFunc(600*time.Millisecond, func() {
		g.window.Show()
		g.window.Focus()
		windowVisible.Store(true)
	})
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
func (g *GuiApp) QuitApp() {
	sendOSNotification("Subs Check Pro", "正在退出…")
	app := application.Get()
	if app != nil {
		app.Quit()
	}
}

// OpenConfigFile 打开系统文件选择对话框，返回用户选择的配置文件路径。
// 用户取消时返回空字符串（不返回错误）。
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
		// 部分平台（Windows）在用户取消时返回错误而非空字符串
		// 检测 cancel 语义，统一处理为"取消 = 空字符串，无错误"
		errLower := strings.ToLower(err.Error())
		if result == "" &&
			(strings.Contains(errLower, "cancel") ||
				strings.Contains(errLower, "cancelled") ||
				strings.Contains(errLower, "no file") ||
				strings.Contains(errLower, "user cancelled")) {
			return "", nil
		}
		return "", fmt.Errorf("打开文件对话框失败: %w", err)
	}
	return result, nil
}

// ── 双窗口调度辅助 ───────────────────────────────────────────────────────────

// showActiveWindow 根据当前所处模式（WebUI / 登录窗口）显示对应窗口。
// 供托盘菜单及单实例唤醒信号调用。
func (g *GuiApp) showActiveWindow() {
	if g.inWebUI.Load() {
		// 当前处于 WebUI 大窗模式
		if g.webUIWin != nil {
			showWindow(g.webUIWin)
		}
	} else {
		// 当前处于登录小窗模式
		if g.loginWin != nil {
			showWindow(g.loginWin)
		}
	}
}

// hideActiveWindow 根据当前所处模式（WebUI / 登录窗口）隐藏对应窗口。
// 供托盘菜单"隐藏界面"调用。
func (g *GuiApp) hideActiveWindow() {
	if g.inWebUI.Load() {
		// 当前处于 WebUI 大窗模式
		if g.webUIWin != nil {
			hideWindow(g.webUIWin)
		}
	} else {
		// 当前处于登录小窗模式
		if g.loginWin != nil {
			hideWindow(g.loginWin)
		}
	}
}

// ── 端口校验辅助 ─────────────────────────────────────────────────────────────

func normalizePort(port string) string {
	return strings.TrimPrefix(strings.TrimSpace(port), ":")
}

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
		return fmt.Errorf("端口 %d 是系统保留端口（1-1023），请使用 1024-65535 范围内的端口", p)
	case p > 65535:
		return fmt.Errorf("端口不能大于 65535，当前值: %d", p)
	}

	return nil
}

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