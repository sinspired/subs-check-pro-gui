/**
 * frontend/src/AboutApp.tsx
 * 「关于」独立窗口 — 侘寂风分栏设计（800×600 优化版）
 *
 * 布局：左侧边栏 200px（Logo + 版本 + 垂直导航）
 *       右侧内容 600px（工具栏 + 面板 + 底栏）
 */
import { useEffect, useState } from 'preact/hooks';
import { useTheme } from '../hooks/useTheme';
import { useWailsReady } from '../hooks/useWailsReady';
import { GuiApp, AppInfo } from '../../bindings/github.com/sinspired/subs-check-pro-gui';
import { useToast } from '../hooks/useToast';
import { Toast } from '../components/Toast';

async function openLink(url: string) {
  try {
    await GuiApp.OpenBrandURL(url);
  } catch {
    window.open(url, '_blank');
  }
}

// ── 类型 ──────────────────────────────────────────────────────────
type Tab = 'intro' | 'features' | 'resources';

// ── 静态数据 ──────────────────────────────────────────────────────
const NAV_ITEMS: { id: Tab; label: string; hint: string }[] = [
  { id: 'intro', label: '系统概览', hint: 'Overview' },
  { id: 'features', label: '核心特性', hint: 'Features' },
  { id: 'resources', label: '生态资源', hint: 'Resources' },
];

const FEATURES = [
  { emoji: '⚡', label: '自适应流水线高并发测试模式' },
  { emoji: '🔋', label: '极致内存调度，支持亿级节点' },
  { emoji: '🗺️', label: 'GeoDB 增强与广播 IP 深度识别' },
  { emoji: '📦', label: '自动生成开箱即用 sing-box 模板' },
  { emoji: '🎲', label: '智能乱序重排与历史存活校验' },
  { emoji: '📊', label: '质量分布与协议类型可视化面板' },
  { emoji: '🚦', label: '局域网代理嗅探与无感防冲突' },
  { emoji: '🧩', label: '深度继承 Sub-Store 节点生态' },
  { emoji: '🔒', label: '内置文件分发，支持独立防盗码' },
  { emoji: '📱', label: '适配多端屏幕的现代 Web UI 看板' },
];

// 资源链接：主推（Telegram）+ 次要三项
const TG_LINK = {
  title: 'Telegram 交流群',
  desc: '实时技术探讨 · Issue 跟踪 · 版本更新推送 · 欢迎加入',
  url: 'https://t.me/subs_check_pro',
};

const SEC_LINKS = [
  {
    svgSrc: '/github.svg' as string | null,
    title: 'GUI 客户端仓库',
    desc: '获取源码、构建版本及主题资源',
    url: 'https://github.com/sinspired/subs-check-pro-gui',
  },
  {
    svgSrc: '/github.svg' as string | null,
    title: '内核仓库',
    desc: '查阅内核引擎源码及官方 Docker 镜像',
    url: 'https://github.com/sinspired/subs-check-pro',
  },
  {
    svgSrc: null,
    title: '官方知识库 Wiki',
    desc: '部署教程、订阅规则及高阶配置',
    url: 'https://github.com/sinspired/subs-check-pro/wiki',
  },
];

// ── 图标 ──────────────────────────────────────────────────────────
const SunIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none"
    stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <circle cx="12" cy="12" r="5" />
    <line x1="12" y1="1" x2="12" y2="3" /><line x1="12" y1="21" x2="12" y2="23" />
    <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
    <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
    <line x1="1" y1="12" x2="3" y2="12" /><line x1="21" y1="12" x2="23" y2="12" />
    <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
    <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
  </svg>
);

const MoonIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none"
    stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
  </svg>
);

// Wiki 书本图标（内联，无需外部文件）
const WikiIcon = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none"
    stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
    <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
  </svg>
);

// 右上角跳转箭头
const ArrowIcon = () => (
  <svg class="aw-link-arrow" width="13" height="13" viewBox="0 0 24 24" fill="none"
    stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <line x1="7" y1="17" x2="17" y2="7" />
    <polyline points="7 7 17 7 17 17" />
  </svg>
);

// ── 组件 ──────────────────────────────────────────────────────────
export function AboutApp() {
  const ready = useWailsReady();
  const { theme, toggleTheme } = useTheme();
  const isDark = theme === 'dark';
  const [info, setInfo] = useState<AppInfo | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>('intro');

  const { msg, visible, toast } = useToast();

  useEffect(() => {
    if (!ready) return;
    GuiApp.GetAppInfo().then(setInfo).catch(() => { });
  }, [ready]);

  const guiVer = info?.guiVersion || 'dev';
  const coreVer = info?.coreVersion || 'dev';

  function switchTab(tab: Tab) {
    if (tab !== activeTab) setActiveTab(tab);
  }

  return (
    <div class="aw-root">

      {/* ══════════════════════════════════════════
          左侧边栏
          ══════════════════════════════════════════ */}
      <aside class="aw-sidebar">

        {/* Mac 标题栏高度留白（拖拽区） */}
        <div class="aw-sidebar-titlebar" />

        {/* Logo + 应用名 */}
        <div class="aw-brand">
          <img src="/logo.svg" alt="Logo" class="aw-logo" />
          <div class="aw-app-name">
            Subs Check <span class="pro">PRO⁺</span>
          </div>
          <div class="aw-app-sub">高并发代理质量检测终端</div>
        </div>

        {/* 版本块 */}
        <div class="aw-ver-block">
          <div
            class="aw-ver-row"
            onClick={() => openLink('https://github.com/sinspired/subs-check-pro-gui')}
            title="打开 GUI 仓库"
          >
            <span class="aw-ver-dot gui-dot" />
            <span class="aw-ver-label">GUI</span>
            <span class="aw-ver-val accent">{guiVer}</span>
          </div>
          <div class="aw-ver-divider" />
          <div
            class="aw-ver-row"
            onClick={() => openLink('https://github.com/sinspired/subs-check-pro')}
            title="打开 Core 仓库"
          >
            <span class="aw-ver-dot core-dot" />
            <span class="aw-ver-label">Core</span>
            <span class="aw-ver-val">{coreVer}</span>
          </div>
        </div>

        {/* 垂直导航 */}
        <nav class="aw-nav">
          {NAV_ITEMS.map(({ id, label, hint }) => (
            <button
              key={id}
              class={`aw-nav-item ${activeTab === id ? 'active' : ''}`}
              onClick={() => switchTab(id)}
            >
              <span class="aw-nav-label">{label}</span>
              <span class="aw-nav-hint">{hint}</span>
            </button>
          ))}
        </nav>

        {/* 底部版权 */}
        <div class="aw-sidebar-footer">
          © 2026 sinspired
        </div>

      </aside>

      {/* ══════════════════════════════════════════
          右侧内容区
          ══════════════════════════════════════════ */}
      <div class="aw-content">

        {/* 工具栏：拖拽 + 主题切换 */}
        <div class="aw-content-toolbar">
          <div class="aw-drag-area" />
          <button class="icon-btn theme-btn" onClick={toggleTheme} title="切换主题">
            {isDark ? <SunIcon /> : <MoonIcon />}
          </button>
        </div>

        {/* 面板容器 */}
        <main class="aw-main">

          {/* ── 概览 ── */}
          {activeTab === 'intro' && (
            <div class="aw-panel" key="intro">
              <p class="aw-intro-lead">
                基于 <strong>Wails v3</strong> 现代化框架构建。告别繁杂的命令行，
                通过极具呼吸感的免配置界面，为底层引擎提供系统级原生适配，
                体验千万级节点的高效自适应测速测活。
              </p>
              <div class="aw-arch-grid">
                <div class="aw-arch-col">
                  <h3><span class="dot gui" />表现层 (GUI)</h3>
                  <ul>
                    <li>Win / Mac / Linux 原生渲染</li>
                    <li>系统托盘与 OS 生命周期集成</li>
                    <li>沉浸式侘寂风与深色模式变幻</li>
                    <li>零配置即刻启动与沙盒化驻留</li>
                  </ul>
                </div>
                <div class="aw-arch-col">
                  <h3><span class="dot core" />通信层 (Core)</h3>
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

          {/* ── 特性 ── */}
          {activeTab === 'features' && (
            <div class="aw-panel" key="features">
              <div class="aw-features-grid">
                {FEATURES.map(({ emoji, label }) => (
                  <div class="aw-feature-item" key={label}>
                    <span class="aw-feature-emoji">{emoji}</span>
                    <span class="aw-feature-label">{label}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* ── 资源 ── */}
          {activeTab === 'resources' && (
            <div class="aw-panel" key="resources">
              <div class="aw-res-layout">

                {/* ── Telegram 主推卡（突出） ── */}
                <div class="aw-link-card aw-featured" onClick={() => openLink(TG_LINK.url)}>
                  <div class="aw-link-icon-wrap aw-featured-icon">
                    <img src="/telegram.svg" class="aw-link-svg" alt="Telegram" />
                  </div>
                  <div class="aw-link-body">
                    <strong class="aw-link-title">{TG_LINK.title}</strong>
                    <span class="aw-link-desc">{TG_LINK.desc}</span>
                  </div>
                  <ArrowIcon />
                </div>

                {/* ── 次要三项（等宽并排） ── */}
                <div class="aw-res-secondary">
                  {SEC_LINKS.map(({ svgSrc, title, desc, url }) => (
                    <div class="aw-link-card aw-compact" key={url} onClick={() => openLink(url)}>
                      <div class="aw-link-icon-wrap">
                        {svgSrc
                          ? <img src={svgSrc} class="aw-link-svg" alt={title} />
                          : <WikiIcon />
                        }
                      </div>
                      <strong class="aw-link-title">{title}</strong>
                      <span class="aw-link-desc">{desc}</span>
                      <ArrowIcon />
                    </div>
                  ))}
                </div>

                {/* ── 快速参考：Docker / 链接直达 ── */}
                <div class="aw-quickref">

                  {/* Docker 拉取命令（点击复制） */}
                  <div class="aw-qr-item" onClick={() =>
                    navigator.clipboard.writeText('docker pull sinspired/subs-check-pro')
                      .then(() => toast('已复制 Docker 命令'))
                  }>
                    <span class="aw-qr-label">Docker</span>
                    <span class="aw-qr-val aw-qr-cmd">docker pull sinspired/subs-check-pro</span>
                    {/* 复制图标 */}
                    <svg class="aw-qr-action" width="11" height="11" viewBox="0 0 24 24" fill="none"
                      stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                    </svg>
                  </div>

                  {/* 通知渠道配置教程 */}
                  <div class="aw-qr-item" onClick={() =>
                    openLink('https://github.com/sinspired/subs-check-pro/wiki/Notifications')
                  }>
                    <span class="aw-qr-label">通知配置</span>
                    <span class="aw-qr-val">github.com/sinspired/subs-check-pro/wiki/Notifications</span>
                    <svg class="aw-qr-action" width="11" height="11" viewBox="0 0 24 24" fill="none"
                      stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                      <polyline points="15 3 21 3 21 9" /><line x1="10" y1="14" x2="21" y2="3" />
                    </svg>
                  </div>

                  {/* 现代 WebUI 管理界面 */}
                  <div class="aw-qr-item" onClick={() => openLink('http://localhost:8199/admin')}>
                    <span class="aw-qr-label">WebUI</span>
                    <span class="aw-qr-val">localhost:8199/admin</span>
                    <svg class="aw-qr-action" width="11" height="11" viewBox="0 0 24 24" fill="none"
                      stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                      <polyline points="15 3 21 3 21 9" /><line x1="10" y1="14" x2="21" y2="3" />
                    </svg>
                  </div>
                  {/* 检测结果分析报告 */}
                  <div class="aw-qr-item" onClick={() => openLink('http://localhost:8199/analysis')}>
                    <span class="aw-qr-label">分析报告</span>
                    <span class="aw-qr-val">localhost:8199/analysis</span>
                    <svg class="aw-qr-action" width="11" height="11" viewBox="0 0 24 24" fill="none"
                      stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                      <polyline points="15 3 21 3 21 9" /><line x1="10" y1="14" x2="21" y2="3" />
                    </svg>
                  </div>
                  {/* 内置文件服务 */}
                  <div class="aw-qr-item" onClick={() => openLink('http://localhost:8199/files')}>
                    <span class="aw-qr-label">文件服务</span>
                    <span class="aw-qr-val">localhost:8199/files</span>
                    <svg class="aw-qr-action" width="11" height="11" viewBox="0 0 24 24" fill="none"
                      stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                      <polyline points="15 3 21 3 21 9" /><line x1="10" y1="14" x2="21" y2="3" />
                    </svg>
                  </div>
                  {/* 自建 GitHub 加速代理 */}
                  <div class="aw-qr-item" onClick={() => openLink('https://github.com/sinspired/CF-Proxy')}>
                    <span class="aw-qr-label">CF Proxy</span>
                    <span class="aw-qr-val">github.com/sinspired/CF-Proxy</span>
                    <svg class="aw-qr-action" width="11" height="11" viewBox="0 0 24 24" fill="none"
                      stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                      <polyline points="15 3 21 3 21 9" /><line x1="10" y1="14" x2="21" y2="3" />
                    </svg>
                  </div>

                </div>
              </div>
            </div>
          )}

        </main>

        {/* 底栏 */}
        <footer class="aw-content-footer">
          仅供学习与调试研究，请遵守相关法律法规。
        </footer>

      </div>
      <Toast msg={msg} visible={visible} />
    </div>
  );
}