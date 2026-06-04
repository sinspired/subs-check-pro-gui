/**
 * frontend/src/AboutApp.tsx
 * 「关于」独立窗口 - 纯粹的侘寂风与极简设计
 */
import { useEffect, useState } from 'preact/hooks';
import { useTheme } from '../hooks/useTheme';
import { useWailsReady } from '../hooks/useWailsReady';
import { GuiApp, AppInfo } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

async function openLink(url: string) {
  try {
    await GuiApp.OpenBrandURL(url);
  } catch {
    window.open(url, '_blank');
  }
}

// 极简线框图标
const IconApp = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
    <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>
  </svg>
);
const IconTerminal = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
    <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
  </svg>
);

export function AboutApp() {
  const ready = useWailsReady();
  const { theme, toggleTheme } = useTheme();
  const isDark = theme === 'dark';
  
  const [info, setInfo] = useState<AppInfo | null>(null);
  const [activeTab, setActiveTab] = useState<'intro' | 'features' | 'resources'>('intro');

  useEffect(() => {
    if (!ready) return;
    GuiApp.GetAppInfo().then(setInfo).catch(() => {});
  }, [ready]);

  const guiVer = info?.guiVersion || 'dev';
  const coreVer = info?.coreVersion || 'dev';

  return (
    <div class="aw-root">

      {/* ── 顶部拖拽区与主题切换 ── */}
      <div class="aw-toolbar">
        <div class="aw-drag" />
        <button class="icon-btn theme-btn" onClick={toggleTheme} title="切换主题">
          {!isDark ? (
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
          ) : (
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>
          )}
        </button>
      </div>

      {/* ── 头部 Logo、标题与微章 ── */}
      <header class="aw-header">
        <img src="/logo.svg" alt="Logo" class="aw-logo" />
        <div class="aw-title">Subs Check <span class="pro">PRO⁺</span></div>
        <div class="aw-subtitle">高并发网络代理质量检测终端</div>
        
        <div class="aw-badges">
          <div class="aw-badge gui" onClick={() => openLink("https://github.com/sinspired/subs-check-pro-gui")}>
            <IconApp /> {guiVer}
          </div>
          <div class="aw-badge core" onClick={() => openLink("https://github.com/sinspired/subs-check-pro")}>
            <IconTerminal /> Core {coreVer}
          </div>
        </div>
      </header>

      {/* ── 禅意文字导航 ── */}
      <nav class="aw-nav">
        <button class={`aw-nav-item ${activeTab === 'intro' ? 'active' : ''}`} onClick={() => setActiveTab('intro')}>系统概览</button>
        <button class={`aw-nav-item ${activeTab === 'features' ? 'active' : ''}`} onClick={() => setActiveTab('features')}>核心特性</button>
        <button class={`aw-nav-item ${activeTab === 'resources' ? 'active' : ''}`} onClick={() => setActiveTab('resources')}>生态资源</button>
      </nav>

      {/* ── 视图面板 ── */}
      <main class="aw-main">
        
        {activeTab === 'intro' && (
          <div class="aw-panel">
            <div class="aw-intro-text">
              基于 <strong>Wails v3</strong> 现代化框架构建。告别繁杂的命令行，通过极具呼吸感的免配置界面，<br/>为底层引擎提供系统级原生适配，体验千万级节点的高效自适应测速测活。
            </div>
            <div class="aw-arch-grid">
              <div class="aw-arch-col">
                <h3><span class="dot gui"></span>表现层 (GUI)</h3>
                <ul>
                  <li>Win / Mac / Linux 原生渲染</li>
                  <li>系统托盘与 OS 生命周期集成</li>
                  <li>沉浸式侘寂风与深色模式变幻</li>
                  <li>零配置即刻启动与沙盒化驻留</li>
                </ul>
              </div>
              <div class="aw-arch-col">
                <h3><span class="dot core"></span>通信层 (Core)</h3>
                <ul>
                  <li>高并发自适应流与极限内存控制</li>
                  <li>智能局域网嗅探与乱序纠错纠偏</li>
                  <li>多面板：GUI 终端与现代 WebUI</li>
                  <li>原生 GeoDB 增强与广播 IP 识别</li>
                </ul>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'features' && (
          <div class="aw-panel">
            <div class="aw-features-grid">
              <div class="aw-feature-item"><span class="aw-feature-emoji">⚡</span>自适应流水线高并发测试模式</div>
              <div class="aw-feature-item"><span class="aw-feature-emoji">🔋</span>极致内存调度策略，支持亿级节点</div>
              <div class="aw-feature-item"><span class="aw-feature-emoji">🗺️</span>GeoDB 原生增强与广播 IP 深度识别</div>
              <div class="aw-feature-item"><span class="aw-feature-emoji">📦</span>自动生成开箱即用的 sing-box 模板</div>
              <div class="aw-feature-item"><span class="aw-feature-emoji">🎲</span>智能节点乱序重排与历史存活校验</div>
              <div class="aw-feature-item"><span class="aw-feature-emoji">📊</span>质量分布、协议类型占比可视化面板</div>
              <div class="aw-feature-item"><span class="aw-feature-emoji">🚦</span>局域网代理智能嗅探与无感防冲突机制</div>
              <div class="aw-feature-item"><span class="aw-feature-emoji">🧩</span>深度继承 Sub-Store 节点管理生态</div>
              <div class="aw-feature-item"><span class="aw-feature-emoji">🔒</span>内置极简文件分发，支持独立防盗码</div>
              <div class="aw-feature-item"><span class="aw-feature-emoji">📱</span>适配多端屏幕尺寸的现代 Web UI 看板</div>
            </div>
          </div>
        )}

        {activeTab === 'resources' && (
          <div class="aw-panel">
            <div class="aw-links-grid">
              <a onClick={() => openLink("https://github.com/sinspired/subs-check-pro-gui")} class="aw-link-card" href="javascript:void(0)">
                <div class="aw-link-icon">🖥️</div>
                <div class="aw-link-text">
                  <strong>GUI 客户端仓库</strong>
                  <span>获取源码、构建版本及主题资源</span>
                </div>
              </a>
              <a onClick={() => openLink("https://github.com/sinspired/subs-check-pro")} class="aw-link-card" href="javascript:void(0)">
                <div class="aw-link-icon">⚙️</div>
                <div class="aw-link-text">
                  <strong>Core 核心主页</strong>
                  <span>查阅引擎源码及官方 Docker 镜像</span>
                </div>
              </a>
              <a onClick={() => openLink("https://github.com/sinspired/subs-check-pro/wiki")} class="aw-link-card" href="javascript:void(0)">
                <div class="aw-link-icon">📖</div>
                <div class="aw-link-text">
                  <strong>官方知识库 Wiki</strong>
                  <span>部署教程、订阅规则及高阶配置</span>
                </div>
              </a>
              <a onClick={() => openLink("https://t.me/subs_check_pro")} class="aw-link-card" href="javascript:void(0)">
                <div class="aw-link-icon">💬</div>
                <div class="aw-link-text">
                  <strong>Telegram 交流群</strong>
                  <span>技术探讨、Issue 跟踪与版本推送</span>
                </div>
              </a>
            </div>
          </div>
        )}
      </main>

      {/* ── 极简底栏 ── */}
      <footer class="aw-footer">
        <span>© 2026 sinspired. All rights reserved.</span>
        <span>仅供学习与调试研究，请遵守相关法律法规。</span>
      </footer>

    </div>
  );
}