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
  { emoji: '📱', label: '现代 Web UI 和 跨平台桌面客户端' },
  { emoji: '⚡', label: '自适应流水线高并发测试模式' },
  { emoji: '🔋', label: '极致内存调度，千万节点低内存占用' },
  { emoji: '🗺️', label: 'GeoDB 增强地理位置标签' },
  { emoji: '📡', label: 'ISP / 原生 IP 类型检测' },
  { emoji: '📦', label: '自动生成开箱即用 sing-box 订阅' },
  { emoji: '📦', label: '自动生成开箱即用 mihomo 订阅' },
  { emoji: '🎲', label: '智能乱序重排' },
  { emoji: '🕒', label: '历史可用节点缓存复用' },
  { emoji: '📊', label: '检测结果分析报告 & 位置与协议分布可视化' },
  { emoji: '🚦', label: '自动检测系统代理环境' },
  { emoji: '🧩', label: '深度集成 Sub-Store 前后端' },
  { emoji: '🔒', label: '内置文件分发，支持独立防盗码' },
  { emoji: '📣', label: '多渠道消息通知推送' },
  { emoji: '🚦', label: '自动检测系统代理环境' },
  { emoji: '🎁', label: '自动无缝版本更新' },
  { emoji: '✏️', label: '配置编辑器 & 自动补全' },
  { emoji: '🔗', label: '多种非标订阅格式超级解码' },
  { emoji: '6️⃣', label: '支持 IPv6 代理节点' },
  { emoji: '🛠️', label: '开源免费，社区驱动持续迭代' },
];

// 资源链接：主推（Telegram）+ 次要三项
const TG_LINK = {
  title: 'Telegram 群组',
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
    title: '内核引擎仓库',
    desc: '查阅内核引擎源码，版本发布及官方 Docker 镜像',
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
            title="打开 内核 仓库"
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
          © 2026{' '}
          <span
            style="cursor:pointer; text-decoration:underline; text-underline-offset:2px;"
            onClick={() => openLink('https://github.com/sinspired')}
          >
            Sinspired
          </span>
          <span>&nbsp;·&nbsp;</span>
          <span
            style="cursor:pointer; text-decoration:underline; text-underline-offset:2px;"
            onClick={() => openLink('https://www.gnu.org/licenses/gpl-3.0.html')}
          >
            GPL-3.0 License
          </span>
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

              {/* 引言：从 README 精炼 */}
              <p class="aw-intro-lead">
                基于 <strong>Wails v3</strong> 现代化框架构建，为底层引擎提供系统级原生适配。支持 <strong>定时检测</strong> 任务，自动生成 <strong>mihomo</strong> 与 <strong>sing-box</strong> 订阅，一键复制订阅链接。
              </p>

              {/* 双栏架构 */}
              <div class="aw-arch-grid">
                <div class="aw-arch-col">
                  <h3><span class="dot gui" />桌面客户端</h3>
                  <ul>
                    <li>Win / Mac / Linux 原生渲染</li>
                    <li>系统托盘与 OS 生命周期集成</li>
                    <li>沉浸式侘寂风与深色模式</li>
                    <li>零配置即刻启动与沙盒化驻留</li>
                    <li>Wails v3 跨平台框架</li>
                    <li>React + TypeScript 前端</li>
                    <li>系统级原生窗口适配</li>
                  </ul>
                </div>
                <div class="aw-arch-col">
                  <h3><span class="dot core" />高性能内核</h3>
                  <ul>
                    <li>自适应流水线高并发引擎</li>
                    <li>支持千万级节点池</li>
                    <li>低内存占用</li>
                    <li>现代 WebUI 管理界面</li>
                    <li>Docker 容器部署支持</li>
                    <li>自动无缝版本更新</li>
                    <li>支持 IPv6 代理节点</li>
                  </ul>
                </div>
              </div>

              {/* 本地服务快捷入口（复用 quickref 样式，动态端口） */}
              {(() => {
                // 动态端口，降级 8199
                const port = info?.listenPort || '8199';

                // 外链 SVG：右上角箭头（统一复用）
                const IconExternal = () => (
                  <svg class="aw-qr-action" width="11" height="11" viewBox="0 0 24 24" fill="none"
                    stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                    <polyline points="15 3 21 3 21 9" />
                    <line x1="10" y1="14" x2="21" y2="3" />
                  </svg>
                );
                return (
                  <div class="aw-quickref">
                    <div class="aw-qr-group">本地服务</div>

                    <div class="aw-qr-item" onClick={() => openLink(`http://localhost:${port}/admin`)}>
                      <svg class="aw-qr-icon" width="12" height="12" viewBox="0 0 24 24" fill="none"
                        stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" />
                        <rect x="14" y="14" width="7" height="7" /><rect x="3" y="14" width="7" height="7" />
                      </svg>
                      <span class="aw-qr-label">管理界面</span>
                      <span class="aw-qr-val">localhost:{port}/admin</span>
                      <IconExternal />
                    </div>

                    <div class="aw-qr-item" onClick={() => openLink(`http://localhost:${port}/analysis`)}>
                      <svg class="aw-qr-icon" width="12" height="12" viewBox="0 0 24 24" fill="none"
                        stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <line x1="18" y1="20" x2="18" y2="10" />
                        <line x1="12" y1="20" x2="12" y2="4" />
                        <line x1="6" y1="20" x2="6" y2="14" />
                      </svg>
                      <span class="aw-qr-label">分析报告</span>
                      <span class="aw-qr-val">localhost:{port}/analysis</span>
                      <IconExternal />
                    </div>

                    <div class="aw-qr-item" onClick={() => openLink(`http://localhost:${port}/files`)}>
                      <svg class="aw-qr-icon" width="12" height="12" viewBox="0 0 24 24" fill="none"
                        stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
                      </svg>
                      <span class="aw-qr-label">文件服务</span>
                      <span class="aw-qr-val">localhost:{port}/files</span>
                      <IconExternal />
                    </div>
                  </div>
                );
              })()}

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

                {/* ── 快速参考区 ── */}
                {(() => {
                  // 外链 SVG：右上角箭头（统一复用）
                  const IconExternal = () => (
                    <svg class="aw-qr-action" width="11" height="11" viewBox="0 0 24 24" fill="none"
                      stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                      <polyline points="15 3 21 3 21 9" />
                      <line x1="10" y1="14" x2="21" y2="3" />
                    </svg>
                  );

                  return (
                    <div class="aw-quickref">
                      {/* ── 部署 & 资源 ── */}
                      <div class="aw-qr-group">部署</div>

                      {/* Docker：点击复制命令 */}
                      <div class="aw-qr-item"
                        onClick={() =>
                          navigator.clipboard
                            .writeText('docker pull sinspired/subs-check-pro')
                            .then(() => toast('已复制 Docker 命令'))
                        }>

                        <svg class="aw-qr-icon"
                          width="800px"
                          height="800px"
                          viewBox="0 0 15 15"
                          fill="currentColor"
                          xmlns="http://www.w3.org/2000/svg"
                        >
                          <path
                            d="M0.5 5.5V5H0V5.5H0.5ZM2.5 3.5V3H2V3.5H2.5ZM6.5 1.5V1H6V1.5H6.5ZM8.5 1.5H9V1H8.5V1.5ZM12.5 7.5H12V8H12.5V7.5ZM1 7.5V5.5H0V7.5H1ZM3 7.5V3.5H2V7.5H3ZM2.5 4H8.5V3H2.5V4ZM8 3.5V7.5H9V3.5H8ZM5 7.5V3.5H4V7.5H5ZM7 7.5V1.5H6V7.5H7ZM6.5 2H8.5V1H6.5V2ZM8 1.5V3.5H9V1.5H8ZM13.7361 10H15V9H13.7361V10ZM10 5V5.5H11V5H10ZM12 6.5V7.5H13V6.5H12ZM12.5 8H13.5V7H12.5V8ZM14 8.5V9.5H15V8.5H14ZM13.5 8C13.7761 8 14 8.22386 14 8.5H15C15 7.67157 14.3284 7 13.5 7V8ZM11.5 6C11.7761 6 12 6.22386 12 6.5H13C13 5.67157 12.3284 5 11.5 5V6ZM3 10H4V9H3V10ZM8.5 7H0.5V8H8.5V7ZM0 7.5V8.5H1V7.5H0ZM5.5 14H6.02786V13H5.5V14ZM6.02786 14C8.51265 14 10.8164 12.8096 12.2585 10.8496L11.4531 10.257C10.1974 11.9636 8.19126 13 6.02786 13V14ZM0 8.5C0 11.5376 2.46243 14 5.5 14V13C3.01472 13 1 10.9853 1 8.5H0ZM0.5 6H11.5V5H0.5V6ZM10 5.5C10 6.32843 9.32843 7 8.5 7V8C9.88071 8 11 6.88071 11 5.5H10ZM13.7361 9C12.7762 9 11.9673 9.55817 11.4531 10.257L12.2585 10.8496C12.6423 10.3281 13.1808 10 13.7361 10V9Z"
                            fill="currentColor"
                          />
                        </svg>
                        <span class="aw-qr-label">Docker</span>
                        <span class="aw-qr-val aw-qr-cmd">docker pull sinspired/subs-check-pro</span>
                        {/* 复制图标 */}
                        <svg class="aw-qr-action" width="11" height="11" viewBox="0 0 24 24" fill="none"
                          stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                        </svg>
                      </div>
                      {/* ── 资源 ── */}
                      <div class="aw-qr-group">资源</div>
                      {/* Docker Hub */}
                      <div class="aw-qr-item"
                        onClick={() => openLink('https://hub.docker.com/r/sinspired/subs-check-pro')}>
                        <svg class="aw-qr-icon" width="12" height="12" viewBox="0 0 24 24" fill="none"
                          stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                          <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 2 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
                          <polyline points="3.27 6.96 12 12.01 20.73 6.96" />
                          <line x1="12" y1="22.08" x2="12" y2="12" />
                        </svg>
                        <span class="aw-qr-label">DockerHub</span>
                        <span class="aw-qr-val">sinspired/subs-check-pro</span>
                        <IconExternal />
                      </div>

                      {/* 通知渠道配置 */}
                      <div class="aw-qr-item"
                        onClick={() => openLink('https://github.com/sinspired/subs-check-pro/wiki/Notifications')}>
                        <svg class="aw-qr-icon" width="12" height="12" viewBox="0 0 24 24" fill="none"
                          stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                          <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
                          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
                        </svg>
                        <span class="aw-qr-label">通知配置</span>
                        <span class="aw-qr-val">…/wiki/Notifications</span>
                        <IconExternal />
                      </div>

                      {/* 自建 GitHub 加速代理 */}
                      <div class="aw-qr-item"
                        onClick={() => openLink('https://github.com/sinspired/CF-Proxy')}>
                        <svg class="aw-qr-icon" width="12" height="12" viewBox="0 0 24 24" fill="none"
                          stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                          <circle cx="12" cy="12" r="10" />
                          <line x1="2" y1="12" x2="22" y2="12" />
                          <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
                        </svg>
                        <span class="aw-qr-label">CF 加速</span>
                        <span class="aw-qr-val">sinspired/CF-Proxy</span>
                        <IconExternal />
                      </div>

                    </div>
                  );
                })()}
              </div>
            </div>
          )}

        </main>

        {/* 底栏 */}
        <footer class="aw-content-footer">
          <span
            style="cursor:pointer; text-decoration:underline; text-underline-offset:2px;"
            onClick={() => openLink('https://github.com/sinspired/subs-check-pro-gui')}
          >
            GitHub 仓库
          </span>
          &nbsp;·&nbsp;仅供学习与调试研究，请遵守相关法律法规
        </footer>
      </div>

      <Toast msg={msg} visible={visible} />
    </div>
  );
}