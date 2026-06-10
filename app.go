// app.go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	coreapp "github.com/sinspired/subs-check-pro/v2/app"
	"github.com/sinspired/subs-check-pro/v2/config"
	"github.com/sinspired/subs-check-pro/v2/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/sinspired/subs-check-pro-gui/updater"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// globalGuiApp 包级指针，供 router handler（如 handleGuiBackToLogin）访问。
var globalGuiApp *GuiApp

// GuiApp Wails 绑定结构体。
type GuiApp struct {
	configPath string
	backend    *coreapp.App

	// loginWin 登录小窗（加载 Wails 前端资产）。由 main.go 注入。
	loginWin *application.WebviewWindow
	// webUIWin WebUI 大窗（加载 webui/admin.html，初始隐藏）。由 main.go 注入。
	webUIWin *application.WebviewWindow

	// autostartMenuItem 托盘菜单中"开机自启"菜单项的引用。
	autostartMenuItem *application.MenuItem

	// pendingInit 为 true 时表示端口预检发现冲突，Initialize() 尚未调用。
	pendingInit bool

	// isFirstRun 标记本次启动是否为首次运行（创建了默认配置）
	isFirstRun  bool
	inWebUI     atomic.Bool
	aboutWin    *application.WebviewWindow
	subLinksWin *application.WebviewWindow

	// autostart Wails 跨平台开机自启管理器
	autostart *application.AutostartManager

	// updaterApp 持有 wails App 引用，供 CheckForUpdates 调用 Updater。
	updaterApp *application.App
}

// AppInfo 前端展示所需的应用运行信息。
type AppInfo struct {
	APIKey       string `json:"apiKey"`
	ListenPort   string `json:"listenPort"`
	SubStorePort string `json:"subStorePort"`
	// SubStorePath Sub-Store 后端 API 路径（config.yaml 中的 sub-store-path）。
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
	// OriginVersion 内核版本
	OriginVersion string `json:"originVersion"`
}

// OpenBrandURL 在 Wails 无地址栏窗口中打开品牌 / 社交链接。
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

	capturedURL := url
	application.InvokeAsync(func() {
		// 先加载本地 loading 页（即时显示，无白屏）。
		// Hash 仅供 loading.html 显示目标域名提示，实际跳转由 Go 端 SetURL 完成。
		opts := newPopupOptions("Subs Check Pro", "/loading.html#"+capturedURL, windowSize)
		popup := wailsApp.Window.NewWithOptions(opts)
		popup.Show()
		popup.Center()
		popup.Focus()

		// 300 ms 对于本地静态页足够，且不会出现 JS 导航被 WebView 拦截的问题。
		time.AfterFunc(300*time.Millisecond, func() {
			application.InvokeAsync(func() {
				popup.SetURL(capturedURL)
			})
		})
	})
}

// EnterWebUI 由前端调用：切换到本地 WebUI 大窗，隐藏登录小窗。
func (g *GuiApp) EnterWebUI() {
	if g.webUIWin == nil || g.loginWin == nil {
		return
	}
	g.inWebUI.Store(true)
	g.webUIWin.SetURL("/webui/admin.html")
	g.webUIWin.Show()
	g.webUIWin.Center()
	g.webUIWin.Focus()
	g.loginWin.Hide()
}

// GetAPIKey 返回当前配置的 API Key，供本地 WebUI 页面通过 Wails binding 调用。
func (g *GuiApp) GetAPIKey() string {
	return config.GlobalConfig.APIKey
}

// defaultListenPort 返回当前配置的 HTTP 监听端口号（不含冒号前缀）。
// GlobalConfig 未初始化或端口为空时，回退到内置默认值 "8199"。
func defaultListenPort() string {
	port := strings.TrimPrefix(config.GlobalConfig.ListenPort, ":")
	if port == "" {
		return "8199"
	}
	return port
}

// GetListenPort 返回 Gin HTTP 服务监听的端口号（不含冒号），
// 供 newCombinedAssetHandler 构造反向代理目标地址使用。
func (g *GuiApp) GetListenPort() string {
	return defaultListenPort()
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
	port := defaultListenPort()
	subStorePort := strings.TrimPrefix(config.GlobalConfig.SubStorePort, ":")

	// 冲突状态仅在 pendingInit 阶段有意义，动态查询当前占用情况。
	var conflictHTTP, conflictSubStore bool
	if g.pendingInit && g.backend != nil {
		httpAvail, subAvail := g.backend.CheckPortConflict()
		conflictHTTP, conflictSubStore = !httpAvail, !subAvail
	}

	autostartEnabled, _ := g.autostart.IsEnabled()

	coreVer := Version
	if CurrentCommit != "" && CurrentCommit != "unknown" {
		coreVer = Version + "-" + CurrentCommit
	}

	subStorePath := strings.TrimPrefix(
		strings.TrimSpace(config.GlobalConfig.SubStorePath), "/",
	)

	return AppInfo{
		APIKey:               config.GlobalConfig.APIKey,
		ListenPort:           port,
		SubStorePort:         subStorePort,
		SubStorePath:         subStorePath,
		KeyIsRandom:          os.Getenv("GUI_KEY_IS_RANDOM") == "1",
		IsFirstRun:           g.isFirstRun,
		ConfigPath:           g.configPath,
		PortConflictHTTP:     conflictHTTP,
		PortConflictSubStore: conflictSubStore,
		PendingInit:          g.pendingInit,
		AutostartEnabled:     autostartEnabled,
		GuiVersion:           GuiVersion,
		CoreVersion:          coreVer,
		OriginVersion:        Version,
	}
}

// IsBackendReady 动态查询后端是否已成功初始化并正在运行。
func (g *GuiApp) IsBackendReady() bool {
	return !g.pendingInit && g.backend != nil
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

	// 注意：只调用一次 registerGuiRoutes，不重复调用（重复注册同一路径会 panic）。
	registerGuiRoutes(g.backend.GetRouter())

	utils.OSNotifyHook = func(title, body string) {
		sendOSNotification(title, body)
	}

	go g.backend.Run()

	g.configPath = g.backend.GetConfigPath()
	g.pendingInit = false

	sendOSNotification("Subs Check PRO 内核", "服务已成功启动")
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

// ValidatePort 实时验证单个端口号合法性（格式 + 是否被占用）。
// 返回空字符串表示端口格式正确且当前未被占用。
func (g *GuiApp) ValidatePort(port string) string {
	p := normalizePort(port)
	if err := validatePort(p); err != nil {
		return err.Error()
	}
	if isPortInUse(p) {
		return "已占用"
	}
	return ""
}

// SetPorts 更新端口配置。
func (g *GuiApp) SetPorts(httpPort, subStorePort string) error {
	return g.backend.SetPorts(httpPort, subStorePort)
}

// ShowWindow 供前端主动调用，显示并聚焦主窗口。
func (g *GuiApp) ShowWindow() {
	if g.loginWin == nil {
		return
	}
	g.loginWin.Show()
	g.loginWin.Focus()
	windowVisible.Store(true)
}

// HideToTray 供前端"关闭按钮对话框"选择最小化时调用。
func (g *GuiApp) HideToTray() {
	if g.loginWin == nil {
		return
	}
	hideWindow(g.loginWin)
	sendOSNotification("主程序", "已最小化到系统托盘")
}

// QuitApp 供前端"关闭按钮对话框"选择退出时调用。
func (g *GuiApp) QuitApp() {
	sendOSNotification("主程序", "正在关闭…")
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
func (g *GuiApp) showActiveWindow() {
	if g.inWebUI.Load() {
		if g.webUIWin != nil {
			showWindow(g.webUIWin)
		}
	} else {
		if g.loginWin != nil {
			showWindow(g.loginWin)
		}
	}
}

// hideActiveWindow 根据当前所处模式（WebUI / 登录窗口）隐藏对应窗口。
func (g *GuiApp) hideActiveWindow() {
	if g.inWebUI.Load() {
		if g.webUIWin != nil {
			hideWindow(g.webUIWin)
		}
	} else {
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

// ── 开机自启辅助方法 ──────────────────────────────────────────────────────────

// GetAutoStartEnabled 查询当前开机自启状态（供托盘菜单调用）。
func (g *GuiApp) GetAutoStartEnabled() (bool, error) {
	return g.autostart.IsEnabled()
}

// SetAutoStartEnabled 设置开机自启状态（供托盘菜单内部调用，不重复更新托盘 checkbox）。
// 修复：原实现忽略了 enable 参数，总是调用 Enable()。
func (g *GuiApp) SetAutoStartEnabled(enable bool) error {
	if enable {
		return g.autostart.Enable()
	}
	return g.autostart.Disable()
}

// SetAutoStart 供前端 JS 绑定调用，切换开机自启。
// 成功后同步更新托盘菜单 checkbox，保证两侧状态一致。
func (g *GuiApp) SetAutoStart(enabled bool) error {
	if g.autostartMenuItem != nil {
		g.autostartMenuItem.SetChecked(enabled)
	}
	if enabled {
		return g.autostart.Enable()
	}
	return g.autostart.Disable()
}

// OpenSubStoreUI 在弹出窗口中打开 Sub-Store 订阅管理页面。
func (g *GuiApp) OpenSubStoreUI() {
	subStorePort := strings.TrimPrefix(config.GlobalConfig.SubStorePort, ":")
	if subStorePort == "" {
		return
	}

	baseURL := "http://127.0.0.1:" + subStorePort
	subStorePath := strings.TrimSpace(config.GlobalConfig.SubStorePath)
	var targetURL string
	if subStorePath != "" {
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
		// Sub-Store 运行在本机回环地址，直接加载含 ?api= 参数的目标 URL。
		popup := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:     "Sub-Store — 订阅管理",
			Width:     1200,
			Height:    800,
			MinWidth:  800,
			MinHeight: 600,
			URL:       targetURL,
			Mac:       macWindowOpts(50),
		})
		popup.Show()
		popup.Center()
		popup.Focus()
	})
}

// OpenInternalPage 在新窗口中打开内置 Web 页面（如 /files、/analysis）。
// 通过 /gui/enter?n=<nonce>&redirect=<path> 中转，确保弹出窗口自动写入 API Key。
func (g *GuiApp) OpenInternalPage(path string, title string, windowSize string) {
	listenPort := defaultListenPort()

	baseURL := "http://127.0.0.1:" + listenPort
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	nonce := generateNonce(config.GlobalConfig.APIKey, false)
	targetURL := baseURL + "/gui/enter?n=" + nonce + "&redirect=" + path

	wailsApp := application.Get()
	if wailsApp == nil {
		return
	}

	application.InvokeAsync(func() {
		opts := newPopupOptions("Subs Check Pro — "+title, targetURL, windowSize)
		opts.MinWidth = 800
		opts.MinHeight = 600
		popup := wailsApp.Window.NewWithOptions(opts)
		popup.Show()
		popup.Center()
		popup.Focus()
	})
}

// OpenAboutWindow 打开或聚焦「关于」独立窗口（单例模式）。
func (g *GuiApp) OpenAboutWindow() {
	wailsApp := application.Get()
	if wailsApp == nil {
		return
	}
	application.InvokeAsync(func() {
		if g.aboutWin != nil {
			g.aboutWin.Show()
			g.aboutWin.Focus()
			return
		}
		win := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:          "about",
			Title:         "Subs Check Pro — 关于",
			Width:         800,
			Height:        600,
			MinWidth:      640,
			MinHeight:     480,
			DisableResize: false,
			Frameless:     false,
			URL:           "/about.html",
			Mac:           macWindowOpts(30),
		})
		g.aboutWin = win
		win.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
			g.aboutWin = nil
		})
	})
}

// CheckForUpdates 触发更新检查，在 Wails 更新窗口中展示结果。
// 供前端按钮和托盘菜单调用。
func (g *GuiApp) CheckForUpdates() {
	if g.updaterApp == nil {
		sendOSNotification("Subs Check Pro", "更新检查暂不可用")
		return
	}

	ctx := context.Background()

	updateInfo, err := g.updaterApp.Updater.Check(ctx)
	if err != nil {
		slog.Warn("检查更新失败", "error", err)
		sendOSNotification("更新失败", err.Error())
		return
	}

	if updateInfo == nil {
		// 已经是最新版
		slog.Info("当前已是最新版")
		sendOSNotification("Subs Check Pro GUI", "已经是最新版")
		return
	}

	go func() {
		// TODO: 使用 Check 让用户选择是否下载更新
		if err := g.updaterApp.Updater.CheckAndInstall(context.Background()); err != nil {
			slog.Warn("检查更新失败", "error", err)
			sendOSNotification("更新失败", err.Error())
		} else {
			// FIXME: 如果本身已经是最新版，不应发送通知
			sendOSNotification("更新完成", "新版本已安装，请彻底退出并在重新启动软件后生效。")
		}
	}()
}

// UpdateInfo 前端展示更新状态所需的结构体。
type UpdateInfo struct {
	HasUpdate      bool   `json:"hasUpdate"`
	LatestVersion  string `json:"latestVersion"`
	CurrentVersion string `json:"currentVersion"`
	ReleaseNotes   string `json:"releaseNotes"`
	DownloadURL    string `json:"downloadURL"`
	Error          string `json:"error"`
}

// GetUpdateInfo 向 GitHub API 查询最新 Release，返回更新状态给前端。
func (g *GuiApp) GetUpdateInfo() UpdateInfo {
	const apiURL = "https://api.github.com/repos/sinspired/subs-check-pro-gui/releases/latest"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return UpdateInfo{Error: "构造请求失败: " + err.Error()}
	}
	req.Header.Set("User-Agent", "subs-check-pro-gui/"+GuiVersion)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return UpdateInfo{Error: "网络请求失败: " + err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{Error: fmt.Sprintf("GitHub API 返回 %d", resp.StatusCode)}
	}

	var release struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return UpdateInfo{Error: "解析响应失败: " + err.Error()}
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(GuiVersion, "v")
	if current == "" || current == "dev" {
		current = "0.0.0"
	}

	return UpdateInfo{
		HasUpdate:      semverGreater(latest, current),
		LatestVersion:  release.TagName,
		CurrentVersion: GuiVersion,
		ReleaseNotes:   release.Body,
		DownloadURL:    updater.GhProxyBase + release.HTMLURL,
	}
}

// semverGreater 简单比较两个版本号字符串（格式 "x.y.z"），
// 返回 a > b。不处理预发布标签，仅比较数字段。
func semverGreater(a, b string) bool {
	parse := func(s string) [3]int {
		var parts [3]int
		segs := strings.SplitN(s, ".", 3)
		for i, seg := range segs {
			if i >= 3 {
				break
			}
			// 截断预发布后缀（如 "2-alpha1" → "2"）
			if idx := strings.IndexAny(seg, "-+"); idx >= 0 {
				seg = seg[:idx]
			}
			n, _ := strconv.Atoi(seg)
			parts[i] = n
		}
		return parts
	}
	pa, pb := parse(a), parse(b)
	for i := 0; i < 3; i++ {
		if pa[i] > pb[i] {
			return true
		}
		if pa[i] < pb[i] {
			return false
		}
	}
	return false
}

// OpenSubLinksWindow 打开或聚焦「订阅链接」独立窗口（单例模式）。
func (g *GuiApp) OpenSubLinksWindow() {
	wailsApp := application.Get()
	if wailsApp == nil {
		return
	}
	application.InvokeAsync(func() {
		if g.subLinksWin != nil {
			g.subLinksWin.Show()
			g.subLinksWin.Focus()
			return
		}
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
			Mac:           macWindowOpts(30),
		})
		g.subLinksWin = win
		win.Center()
		win.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
			g.subLinksWin = nil
		})
	})
}

func (g *GuiApp) GetUpdateStatus() updater.UpdateStatus {
	return updater.GetUpdateStatus()
}
