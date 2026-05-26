package main

import (
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/sinspired/subs-check-pro/v2/app"
)

// setupApp 完成业务层初始化，返回三个值供 main() 使用：
//   - coreApp   核心应用实例（用于 Shutdown）
//   - guiApp    Wails 绑定实例（注入窗口引用后传给 Services）
//   - appInitOK 标记业务是否成功启动（退出时决定是否调用 Shutdown）
//
//  1. 先读取配置文件中的端口信息，检测端口是否可用。
//  2. 若端口冲突 → 不调用 coreApp.Initialize()，将 appInitOK=false 返回给 main；
//     Wails 窗口启动后前端读取 guiApp.HasPendingInit()==true，展示端口冲突界面；
//     用户修改端口并点击"应用"后，前端调用 GuiApp.CompleteInit() 完成初始化。
//  3. 若端口正常 → 按原有流程调用 Initialize() + EnsureRouter() + Run()。
func setupApp() (*app.App, *GuiApp, bool) {
	os.Setenv("START_FROM_GUI", "1")

	originVersion := getEnvOrDefault("ORIGIN_VERSION", "dev")
	version := getEnvOrDefault("APP_VERSION", "dev")
	configPath := getEnvOrDefault("CONFIG_PATH", "")

	guiApp := &GuiApp{configPath: configPath}
	coreApp := app.New(originVersion, version, configPath)

	// 端口预检
	// 在 Initialize() 之前尝试读取配置文件中的端口，如果端口已被占用则跳过初始化，
	// 等待前端用户修改端口后再完成初始化（通过 GuiApp.CompleteInit()）。
	if preConflictHTTP, preConflictSub := preCheckPortsFromConfig(configPath); preConflictHTTP || preConflictSub {
		slog.Warn("端口预检发现冲突，跳过 Initialize()，等待前端用户修改端口",
			"httpConflict", preConflictHTTP, "subStoreConflict", preConflictSub)

		guiApp.backend = coreApp
		guiApp.pendingInit = true
		guiApp.preConflictHTTP = preConflictHTTP
		guiApp.preConflictSub = preConflictSub

		return coreApp, guiApp, false
	}

	// 初始化
	if err := coreApp.Initialize(); err != nil {
		if errors.Is(err, app.ErrFirstRun) {
			resolvedPath := coreApp.GetConfigPath()
			slog.Info("首次运行：config.yaml 已创建", "path", resolvedPath)
			os.Setenv("GUI_FIRST_RUN", "1")

			coreApp = app.New(originVersion, version, resolvedPath)
			if err2 := coreApp.Initialize(); err2 != nil {
				slog.Error("首次运行后重新初始化失败", "error", err2)
				os.Exit(1)
			}
			guiApp.configPath = resolvedPath
		} else {
			slog.Error("应用初始化失败，无法启动 GUI", "error", err)
			os.Exit(1)
		}
	} else {
		guiApp.configPath = coreApp.GetConfigPath()
	}

	guiApp.backend = coreApp

	if err := coreApp.EnsureRouter(); err != nil {
		slog.Error("HTTP 路由初始化失败", "error", err)
		os.Exit(1)
	}

	registerGuiAutoLogin(coreApp.GetRouter())

	go coreApp.Run()

	return coreApp, guiApp, true
}

// preCheckPortsFromConfig 在 Initialize() 之前，直接解析配置文件提取端口字段，
// 并检测端口是否已被占用。
//
// 解析失败（文件不存在、格式错误）时保守返回 (false, false)，
// 交由 Initialize() 按正常流程处理首次运行等情况。
func preCheckPortsFromConfig(configPath string) (httpConflict, subConflict bool) {
	path := resolveConfigFilePath(configPath)
	if path == "" {
		return false, false // 配置文件不存在，属于首次运行，不做预检
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}

	// 只解析需要的端口字段，不加载完整配置（避免副作用）
	var partial struct {
		ListenPort   string `yaml:"listen_port"`
		SubStorePort string `yaml:"sub_store_port"`
	}
	if err := yaml.Unmarshal(data, &partial); err != nil {
		return false, false
	}

	httpPort := normalizePort(partial.ListenPort)
	if httpPort == "" {
		httpPort = "8199" // 默认端口
	}
	httpConflict = isPortInUse(httpPort)

	if partial.SubStorePort != "" {
		subPort := normalizePort(partial.SubStorePort)
		if subPort != "" {
			subConflict = isPortInUse(subPort)
		}
	}

	return httpConflict, subConflict
}

// resolveConfigFilePath 尝试确定配置文件的实际路径。
// 返回空字符串表示找不到文件（属于首次运行，不需要预检）。
func resolveConfigFilePath(hint string) string {
	// 1. 优先使用命令行/环境变量指定的路径
	candidates := []string{hint}

	// 2. 常见默认路径
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			home+"/.config/subs-check-pro/config.yaml",
			home+"/.config/subs-check-pro/config.yml",
		)
	}
	candidates = append(candidates,
		"./config.yaml",
		"./config.yml",
		"./subs-check-pro.yaml",
	)

	for _, p := range candidates {
		if p == "" {
			continue
		}
		p = strings.TrimSpace(p)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
