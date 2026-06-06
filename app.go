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
	"github.com/sinspired/subs-check-pro/v2/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// globalGuiApp 包级指针，供 router handler（如 handleGuiBackToLogin）访问。
var globalGuiApp *GuiApp

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

	// autostartMenuItem 托盘菜单中"开机自启"菜单项的引用。
	// 由 tray.go 的 buildTrayMenu 在创建菜单项后注入，
	// 供前端调用 SetAutoStart 时同步回托盘 checkbox 状态。
	autostartMenuItem *application.MenuItem

	// pendingInit 为 true 时表示端口预检发现冲突，Initialize() 尚未调用。
	pendingInit     bool
	preConflictHTTP bool
	preConflictSub  bool

	// inWebUI 为 true 表示窗口已切换到外部 WebUI 页面。
	// 此时 Wails JS runtime 不可用，关闭事件须走 Go 原生对话框。
	inWebUI atomic.Bool

	// aboutWin 「关于」独立窗口，单例引用。
	// nil 表示窗口已关闭或尚未创建；OpenAboutWindow 负责创建和复用。
	aboutWin *application.WebviewWindow

	// subLinksWin 「订阅链接」独立窗口，单例引用。
	// nil 表示窗口已关闭或尚未创建；OpenSubLinksWindow 负责创建和复用。
	subLinksWin *application.WebviewWindow
}

// AppInfo 前端展示所需的应用运行信息。
type AppInfo struct {
	APIKey       string `json:"apiKey"`
	ListenPort   string `json:"listenPort"`
	SubStorePort string `json:"subStorePort"`
	// SubStorePath Sub-Store 后端 API 路径（config.yaml 中的 sub-store-path）。
	// 前端拼接 ?api=<path> 时使用；若未配置则为空字符串。
	SubStorePath string `json:"subStorePath"`
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
	// GuiVersion 桌面客户端版本（ldflags 注入，如 "v1.2.0"）
	GuiVersion string `json:"guiVersion"`
	// CoreVersion 内核版本+短提交哈希（如 "v2.5.4@7c23868"）
	CoreVersion string `json:"coreVersion"`
}

// OpenBrandURL 在 Wails 无地址栏窗口中打开品牌 / 社交链接。
// 前端品牌面板（GitHub、Telegram、Docker Hub）及版本标签点击时调用，
// 替代 window.open，避免打开系统默认浏览器，保持应用内体验一致。
func (g *GuiApp) OpenBrandURL(url string, windowSize string) {
	if url == "" {
		return
	}
	// 安全校验：只允许 http/https 协议
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return
	}
	wailsApp := application.Get()
	if wailsApp == nil {
		return
	}

	width := 1200
	height := 800

	switch windowSize {
	case "extraLarge":
		width = 1920
		height = 1440
	case "large":
		width = 1600
		height = 1200
	case "medium":
		width = 1200
		height = 800
	case "small":
		width = 720
		height = 720
	case "tiny":
		width = 600
		height = 600
	case "wide":
		width = 1600
		height = 900
	}

	// application.InvokeAsync 确保窗口创建在 Wails 主线程执行
	capturedURL := url
	application.InvokeAsync(func() {
		// 先加载本地 loading 页（即时显示，无白屏）。
		// Hash 仅供 loading.html 显示目标域名提示，实际跳转由 Go 端 SetURL 完成：
		// Wails3 的 WebView 会拦截从本地 wails:// origin 发起的外部 JS 导航，
		// 而 Go 端调用 SetURL 属于宿主进程指令，不经过 JS 导航拦截。
		loadingURL := "/loading.html#" + capturedURL
		popup := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:     "Subs Check Pro",
			Width:     width,
			Height:    height,
			MinWidth:  580,
			MinHeight: 580,
			URL:       loadingURL,
			Mac: application.MacWindow{
				InvisibleTitleBarHeight: 50,
				Backdrop:                application.MacBackdropTranslucent,
				TitleBar:                application.MacTitleBarHiddenInset,
			},
		})
		popup.Show()
		popup.Center()
		popup.Focus()

		// 等待 loading.html 渲染完成后，由 Go 端发起外部 URL 导航。
		// 300 ms 对于本地静态页足够，且不会出现 JS 导航被 WebView 拦截的问题。
		finalURL := capturedURL
		time.AfterFunc(300*time.Millisecond, func() {
			application.InvokeAsync(func() {
				popup.SetURL(finalURL)
			})
		})
	})
}

// EnterWebUI 由前端调用：切换到本地 WebUI 大窗，隐藏登录小窗。
//
// 迁移后不再需要传入 Gin 的 enterURL：
//  - webUIWin 直接加载 Wails 资产服务器上的 /webui/admin.html
//  - APIKey 和端口由 admin.html 内联脚本通过 Wails binding 自行获取
func (g *GuiApp) EnterWebUI() {
	if g.webUIWin == nil || g.loginWin == nil {
		return
	}
	g.inWebUI.Store(true)
	// 加载本地 webui（由 Wails 资产服务器的 newCombinedAssetHandler 提供）
	g.webUIWin.SetURL("/webui/admin.html")
	g.webUIWin.Show()
	g.webUIWin.Center()
	g.webUIWin.Focus()
	// ✅ 打开开发者工具（仅开发模式使用，生产环境建议去掉）
	g.webUIWin.OpenDevTools()
	g.loginWin.Hide()
}

// GetApiKey 返回当前配置的 API Key，供本地 WebUI 页面通过 Wails binding 调用。
//
// 安全边界：该 binding 仅对 Wails 资产服务器提供的页面可见（/webui/admin.html），
// 外部网络无法调用。APIKey 本身已明文保存在 config.yaml，此处不增加额外泄露面。
func (g *GuiApp) GetApiKey() string {
	return config.GlobalConfig.APIKey
}

// GetListenPort 返回 Gin HTTP 服务监听的端口号（不含冒号），
// 供 newCombinedAssetHandler 构造反向代理目标地址使用。
func (g *GuiApp) GetListenPort() string {
	port := strings.TrimPrefix(config.GlobalConfig.ListenPort, ":")
	if port == "" {
		port = "8199"
	}
	return port
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
	g.loginWin.OpenDevTools()
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

	autostartEnabled, _ := queryAutoStart()

	coreVer := Version
	if CurrentCommit != "" && CurrentCommit != "unknown" {
		coreVer = Version + "-" + CurrentCommit
	}

	// Sub-Store 后端路径：去掉可能存在的前导 "/" 后规范化，保持与 JS 侧一致
	subStorePath := strings.TrimPrefix(
		strings.TrimSpace(config.GlobalConfig.SubStorePath), "/",
	)

	return AppInfo{
		APIKey:               config.GlobalConfig.APIKey,
		ListenPort:           port,
		SubStorePort:         subStorePort,
		SubStorePath:         subStorePath,
		KeyIsRandom:          os.Getenv("GUI_KEY_IS_RANDOM") == "1",
		IsFirstRun:           os.Getenv("GUI_FIRST_RUN") == "1",
		ConfigPath:           g.configPath,
		PortConflictHTTP:     conflictHTTP,
		PortConflictSubStore: conflictSubStore,
		PendingInit:          g.pendingInit,
		AutostartEnabled:     autostartEnabled,
		GuiVersion:           GuiVersion,
		CoreVersion:          coreVer,
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

	// 延迟初始化路径同样需要注入 hook
	utils.OSNotifyHook = func(title, body string) {
		sendOSNotification(title, body)
	}

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

// ── 开机自启辅助方法（平台实现在 autostart_*.go）────────────────

// GetAutoStartEnabled 查询当前开机自启状态（供托盘菜单调用）。
func (g *GuiApp) GetAutoStartEnabled() (bool, error) {
	return queryAutoStart()
}

// SetAutoStartEnabled 设置开机自启状态（供托盘菜单内部调用，不重复更新托盘 checkbox）。
func (g *GuiApp) SetAutoStartEnabled(enable bool) error {
	return applyAutoStart(enable)
}

// SetAutoStart 供前端 JS 绑定调用，切换开机自启。
// 成功后同步更新托盘菜单 checkbox，保证两侧状态一致。
func (g *GuiApp) SetAutoStart(enable bool) error {
	if err := applyAutoStart(enable); err != nil {
		return err
	}
	// 同步托盘菜单项 checkbox（若托盘已初始化）
	if g.autostartMenuItem != nil {
		g.autostartMenuItem.SetChecked(enable)
	}
	return nil
}

// OpenSubStoreUI 在弹出窗口中打开 Sub-Store 订阅管理页面。
//
// 设计要点：
//   - 若 config.yaml 配置了 sub-store-path，自动拼接 ?api=<path>，
//     让 Sub-Store 前端直接完成后端绑定，无需用户手动输入。
//   - 窗口先加载本地 loading.html（立即显示，无白屏），300 ms 后由 Go 端通过
//     SetURL 发起外部导航——规避 Wails3 WKWebView/WebView2 对 JS 跨 origin
//     导航的拦截，确保最终页面能正确加载。
//   - 不依赖 JS window.location / window.open，无 WebKit 弹窗拦截问题。
func (g *GuiApp) OpenSubStoreUI() {
	subStorePort := strings.TrimPrefix(config.GlobalConfig.SubStorePort, ":")
	if subStorePort == "" {
		return
	}

	// ── 构建目标 URL ───────────────────────────────────────────────────────
	// Sub-Store 前端首次访问需要 ?api=<backendPath> 才能自动绑定后端，
	// 否则会弹出"请输入后端地址"对话框。
	// 若用户已在 config.yaml 中配置 sub-store-path，则在此处自动附带；
	// 若为随机生成路径（未写入 yaml），则回退到根路径——用户在 Sub-Store
	// 界面手动输入一次后，Sub-Store 会将配置持久化到自己的 localStorage，
	// 后续访问无需再传 ?api=。
	baseURL := "http://127.0.0.1:" + subStorePort
	subStorePath := strings.TrimSpace(config.GlobalConfig.SubStorePath)
	var targetURL string
	if subStorePath != "" {
		// 确保路径以 "/" 开头，格式与 JS 侧一致
		if !strings.HasPrefix(subStorePath, "/") {
			subStorePath = "/" + subStorePath
		}
		targetURL = baseURL + "?api=" + subStorePath
	} else {
		targetURL = baseURL
	}

	wailsApp := application.Get()
	if wailsApp == nil {
		return
	}

	application.InvokeAsync(func() {
		// loading.html 的 hash 仅用于显示目标主机名提示；
		// 实际跳转由 300ms 后的 Go 端 SetURL 完成（规避 JS 跨 origin 导航拦截）。
		popup := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:     "Sub-Store — 订阅管理",
			Width:     1200,
			Height:    800,
			MinWidth:  800,
			MinHeight: 600,
			URL:       "/loading.html#" + baseURL, // hash 只显示主机名，不含 query
			Mac: application.MacWindow{
				InvisibleTitleBarHeight: 50,
				Backdrop:                application.MacBackdropTranslucent,
				TitleBar:                application.MacTitleBarHiddenInset,
			},
		})
		popup.Show()
		popup.Center()
		popup.Focus()

		// 300 ms 后 Go 端发起真实导航（含 ?api= 参数）
		final := targetURL
		time.AfterFunc(300*time.Millisecond, func() {
			application.InvokeAsync(func() {
				popup.SetURL(final)
			})
		})
	})
}

// OpenInternalPage 在新窗口中打开内置 Web 页面（如 /files、/analysis）。
//
// 设计要点：
//   - 所有内置页面均通过 /gui/enter?n=<nonce>&redirect=<path> 中转，
//     确保新弹出窗口的 sessionStorage 写入正确的 API Key，与打开 admin 一致。
//   - 窗口先加载本地 loading.html（立即显示，无白屏），300 ms 后由 Go 端通过
//     SetURL 发起外部导航——规避 JS 跨 origin 导航拦截。
func (g *GuiApp) OpenInternalPage(path string, title string, windowSize string) {
	listenPort := strings.TrimPrefix(config.GlobalConfig.ListenPort, ":")
	if listenPort == "" {
		listenPort = "8199" // fallback
	}

	baseURL := "http://127.0.0.1:" + listenPort
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 生成一次性 nonce，通过 /gui/enter 自动写入 sessionStorage，
	// 使弹出窗口免于手动输入 API Key（与打开 admin 的行为一致）。
	nonce := generateNonce(config.GlobalConfig.APIKey, false)
	targetURL := baseURL + "/gui/enter?n=" + nonce + "&redirect=" + path

	wailsApp := application.Get()
	if wailsApp == nil {
		return
	}

	width := 1200
	height := 800

	switch windowSize {
	case "extraLarge":
		width = 1920
		height = 1440
	case "large":
		width = 1600
		height = 1200
	case "medium":
		width = 1200
		height = 800
	case "small":
		width = 720
		height = 720
	case "tiny":
		width = 600
		height = 600
	case "wide":
		width = 1600
		height = 900
	}

	application.InvokeAsync(func() {
		popup := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:     "Subs Check Pro — " + title,
			Width:     width,
			Height:    height,
			MinWidth:  800,
			MinHeight: 600,
			URL:       "/loading.html#" + baseURL,
			Mac: application.MacWindow{
				InvisibleTitleBarHeight: 50,
				Backdrop:                application.MacBackdropTranslucent,
				TitleBar:                application.MacTitleBarHiddenInset,
			},
		})
		popup.Show()
		popup.Center()
		popup.Focus()

		final := targetURL
		time.AfterFunc(300*time.Millisecond, func() {
			application.InvokeAsync(func() {
				popup.SetURL(final)
			})
		})
	})
}

// OpenAboutWindow 打开或聚焦「关于」独立窗口（单例模式）。
//
// 调用来源：
//   - 系统托盘「关于」菜单项（tray.go）
//   - 主窗口前端「关于」按钮（about-info-btn）
//
// 使用 application.InvokeAsync 确保所有窗口操作在 Wails 主线程执行，
// 避免从 Go binding 调用线程直接操作 UI 导致的竞态问题。
func (g *GuiApp) OpenAboutWindow() {
	wailsApp := application.Get()
	if wailsApp == nil {
		return
	}
	application.InvokeAsync(func() {
		// 窗口已存在：直接显示并聚焦，不重复创建
		if g.aboutWin != nil {
			g.aboutWin.Show()
			g.aboutWin.Focus()
			return
		}
		// 创建新的「关于」窗口
		win := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:          "about", // 窗口唯一名称
			Title:         "Subs Check Pro — 关于",
			Width:         800,
			Height:        600,
			MinWidth:      640,
			MinHeight:     480,
			DisableResize: false,
			Frameless:     false,
			URL:           "/about.html", // Vite MPA 构建输出的入口
			Mac: application.MacWindow{
				InvisibleTitleBarHeight: 30,
				Backdrop:                application.MacBackdropTranslucent,
				TitleBar:                application.MacTitleBarHiddenInset,
			},
		})
		g.aboutWin = win
		// 窗口关闭时清除单例引用，以便下次重新创建
		win.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
			g.aboutWin = nil
		})
	})
}

// OpenSubLinksWindow 打开或聚焦「订阅链接」独立窗口（单例模式）。
//
// 调用来源：
//   - 主窗口前端快捷按钮区「订阅链接」按钮（KeySection）
//
// 窗口加载 Vite MPA 入口 /sub-links.html，前端自行通过
// Wails 资产代理（/api/...）拉取订阅数据并展示。
func (g *GuiApp) OpenSubLinksWindow() {
	wailsApp := application.Get()
	if wailsApp == nil {
		return
	}
	application.InvokeAsync(func() {
		// 窗口已存在：直接显示并聚焦，不重复创建
		if g.subLinksWin != nil {
			g.subLinksWin.Show()
			g.subLinksWin.Focus()
			return
		}
		// 创建新的「订阅链接」窗口（小窗，500×420）
		win := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:          "sub-links",
			Title:         "Subs Check Pro — 订阅链接",
			Width:         500,
			Height:        500,
			MinWidth:      500,
			MinHeight:     500,
			MaxWidth:      500,
			MaxHeight:     520,
			DisableResize: false,
			Frameless:     false,
			URL:           "/sub-links.html",
			Mac: application.MacWindow{
				InvisibleTitleBarHeight: 30,
				Backdrop:                application.MacBackdropTranslucent,
				TitleBar:                application.MacTitleBarHiddenInset,
			},
		})
		g.subLinksWin = win
		win.Center()
		// 窗口关闭时清除单例引用，以便下次重新创建
		win.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
			g.subLinksWin = nil
		})
	})
}
