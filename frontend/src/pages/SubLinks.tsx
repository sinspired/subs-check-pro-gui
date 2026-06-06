/**
 * frontend/src/pages/SubLinks.tsx
 * 订阅链接独立窗口（500×420）
 *
 * 通过 Wails 资产代理（相对路径 /api/...）拉取订阅数据，
 * 展示可一键复制的订阅链接列表。
 * 与 KeySection 中的 fetchSubLinks 共享相同的请求逻辑。
 */
import { useEffect, useState } from 'preact/hooks';
import { useTheme } from '../hooks/useTheme';
import { useWailsReady } from '../hooks/useWailsReady';
import { GuiApp } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

interface SubLink {
  key: string;
  label: string;
  url: string;
}

type Status = 'loading' | 'ready' | 'error';

export function SubLinks() {
  useTheme();
  const wailsReady = useWailsReady();

  const [status, setStatus] = useState<Status>('loading');
  const [errorMsg, setErrorMsg] = useState('');
  const [links, setLinks] = useState<SubLink[]>([]);
  // key → 是否刚刚复制成功（用于短暂高亮）
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  useEffect(() => {
    if (!wailsReady) return;
    load();
  }, [wailsReady]);

  async function load() {
    setStatus('loading');
    setErrorMsg('');
    try {
      const info = await GuiApp.GetAppInfo();

      const headers: Record<string, string> = { 'X-API-Key': info.apiKey };

      // 1. 检查 Sub-Store 是否运行
      const statusRes = await fetch('/api/status', { headers }).catch(() => null);
      if (!statusRes?.ok) throw new Error('获取状态失败，请检查服务是否运行');
      const statusData = await statusRes.json();
      if (!statusData?.isSubStoreRunning)
        throw new Error('Sub-Store 服务未运行，请在配置中启用');

      // 2. 获取 singbox 版本
      const vRes = await fetch('/api/singbox-versions', { headers }).catch(() => null);
      if (!vRes?.ok) throw new Error('获取 singbox 版本失败');
      const vData = await vRes.json();

      // 3. 校验 sub-store-path
      if (!info.subStorePath)
        throw new Error('请先在 config.yaml 中设置 sub-store-path');

      const path = `/${info.subStorePath}`;
      const subBase = `http://127.0.0.1:${info.subStorePort}`;

      setLinks([
        { key: 'common',        label: '通用订阅',                url: `${subBase}${path}/download/sub` },
        { key: 'v2ray',         label: 'V2Ray 订阅',              url: `${subBase}${path}/download/sub?target=V2Ray` },
        { key: 'mihomo',        label: 'Mihomo 订阅',             url: `${subBase}${path}/api/file/mihomo` },
        { key: 'singbox-old',   label: `singbox-${vData.old} 订阅`,   url: `${subBase}${path}/api/file/singbox-${vData.old}` },
        { key: 'singbox-latest',label: `singbox-${vData.latest} 订阅`, url: `${subBase}${path}/api/file/singbox-${vData.latest}` },
      ]);
      setStatus('ready');
    } catch (e: any) {
      setErrorMsg(e?.message ?? '获取订阅链接失败');
      setStatus('error');
    }
  }

  async function copyLink(item: SubLink) {
    try {
      await navigator.clipboard.writeText(item.url);
      setCopiedKey(item.key);
      setTimeout(() => setCopiedKey(k => (k === item.key ? null : k)), 1500);
    } catch {
      // 剪贴板失败时静默忽略，UI 无变化
    }
  }

  return (
    <div class="sl-root">
      {/* ── 标题栏 ── */}
      <div class="sl-titlebar">
        <svg class="sl-titlebar-icon" width="14" height="14" viewBox="0 0 24 24"
          fill="none" stroke="currentColor" stroke-width="2"
          stroke-linecap="round" stroke-linejoin="round">
          <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
          <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
        </svg>
        <span class="sl-titlebar-title">订阅链接</span>
        <span class="sl-titlebar-sub">点击复制订阅链接</span>
      </div>

      {/* ── 内容区 ── */}
      <div class="sl-body">
        {status === 'loading' && (
          <div class="sl-status">
            <svg class="sl-spinner" width="15" height="15" viewBox="0 0 24 24"
              fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 12a9 9 0 1 1-6.219-8.56" />
            </svg>
            正在获取订阅链接…
          </div>
        )}

        {status === 'error' && (
          <div class="sl-status error">
            <span class="sl-error-icon">⚠️</span>
            <span>{errorMsg}</span>
            <button
              style="margin-top:6px;padding:4px 12px;font-size:11px;cursor:pointer;border:1px solid var(--border);border-radius:4px;background:var(--bg);color:var(--fg);"
              onClick={load}
            >
              重试
            </button>
          </div>
        )}

        {status === 'ready' && links.map((item, i) => {
          const copied = copiedKey === item.key;
          return (
            <div
              key={item.key}
              class={`sl-item${copied ? ' copied' : ''}`}
              style={`animation-delay:${i * 35}ms`}
              onClick={() => copyLink(item)}
              title={item.url}
            >
              {/* 图标：复制前链接图标，复制后对勾 */}
              <div class="sl-item-icon">
                {copied ? (
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none"
                    stroke="currentColor" stroke-width="2.5"
                    stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                ) : (
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none"
                    stroke="currentColor" stroke-width="2"
                    stroke-linecap="round" stroke-linejoin="round">
                    <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                    <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
                  </svg>
                )}
              </div>

              <div class="sl-item-info">
                <div class="sl-item-label">{item.label}</div>
                <div class="sl-item-url">{item.url}</div>
              </div>

              <div class="sl-item-action">
                <span class="sl-copy-label">{copied ? '已复制' : ''}</span>
                {!copied && (
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none"
                    stroke="currentColor" stroke-width="2">
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                  </svg>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {/* ── 底部提示 ── */}
      {status === 'ready' && (
        <div class="sl-footer">
          链接指向本机 Sub-Store 端口，请在局域网或本机代理客户端中使用
        </div>
      )}
    </div>
  );
}
