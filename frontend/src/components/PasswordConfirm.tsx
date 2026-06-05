/**
 * frontend/src/components/PasswordConfirm.tsx
 */
import { useState } from 'preact/hooks';
import { GuiApp } from '../../bindings/github.com/sinspired/subs-check-pro-gui';
import { AppInfo } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

interface Props {
  cfgPath: string;
  toast: (msg: string) => void;
  onDone: (newInfo: AppInfo | null) => void;
  onBack: () => void;
  onReselect: (path: string) => void;
}

export function PasswordConfirm({ cfgPath, toast, onDone, onBack, onReselect }: Props) {
  const [key, setKey] = useState('');
  const [loading, setLoading] = useState(false);
  const [keyShown, setKeyShown] = useState(false);

  async function confirm() {
    const trimmed = key.trim();
    if (!trimmed) { toast('请输入 API 密钥'); return; }
    if (loading) return;

    setLoading(true);
    await new Promise<void>(resolve => requestAnimationFrame(() => resolve()));

    try {
      await GuiApp.ValidateConfigKey(trimmed, true);
    } catch (e: any) {
      toast('❌ ' + (e?.message || '密钥错误'));
      setLoading(false);
      return;
    }

    try {
      await GuiApp.EnterWebUI();
    } catch (e: any) {
      toast('进入管理界面失败: ' + (e?.message ?? ''));
      setLoading(false);
    }
  }

  async function handleReselect() {
    let path: string;
    try {
      path = await GuiApp.OpenConfigFile();
    } catch (e: any) {
      toast('打开文件对话框失败: ' + (e?.message ?? '未知错误'));
      return;
    }
    if (!path) return;
    setKey('');        // 清空密码，新配置的密码不同
    onReselect(path);
  }

  return (
    <div class="key-section-flex">

      {/* ── 主体区 ── */}
      <div class="key-body">
        {/* 标签 */}
        <div class="label" style="margin-top:14px">API 密钥</div>

        {/* 密码输入行：复用 key-wrap 样式 */}
        <div class="key-wrap" style="-webkit-app-region:no-drag">
          <input
            class="pw-input"
            type={keyShown ? 'text' : 'password'}
            placeholder="请输入新配置的 API 密钥，回车确认"
            value={key}
            autoFocus
            onInput={e => setKey((e.target as HTMLInputElement).value)}
            onKeyDown={e => { if (e.key === 'Enter') confirm(); }}
          />

          {/* 显示/隐藏 */}
          <button class="icon-btn" onClick={() => setKeyShown(v => !v)}
            title={keyShown ? '隐藏密钥' : '显示密钥'}>
            {!keyShown ? (
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none"
                stroke="currentColor" stroke-width="2">
                <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8
                  a18.45 18.45 0 0 1 5.06-5.94" />
                <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8
                  a18.5 18.5 0 0 1-2.16 3.19" />
                <line x1="1" y1="1" x2="23" y2="23" />
              </svg>
            ) : (
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none"
                stroke="currentColor" stroke-width="2">
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                <circle cx="12" cy="12" r="3" />
              </svg>
            )}
          </button>
        </div>

        {/* 配置文件路径行：与 KeySection 中的样式完全一致 */}
        <div class="cfg-path-row">
          {/* 文件图标 */}
          <svg class="cfg-path-icon" width="11" height="11"
            viewBox="0 0 24 24" fill="none" stroke="currentColor"
            stroke-width="2" stroke-linecap="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
            <polyline points="14 2 14 8 20 8" />
          </svg>

          {/* 路径文本：direction:rtl 使省略号在左侧，保留完整文件名 */}
          <span class="cfg-path-text" title={cfgPath}>{cfgPath}</span>

          {/* 重新选择配置按钮（+ 号） */}
          <button class="icon-btn cfg-path-btn" onClick={handleReselect}
            title="选择其他配置文件">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none"
              stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
              <line x1="12" y1="5" x2="12" y2="19" />
              <line x1="5" y1="12" x2="19" y2="12" />
            </svg>
          </button>
        </div>
      </div>

      {/* 返回登录框（使用默认配置） */}
      <div class="back-btn-row">
        <button class="btn-back" onClick={onBack}
          title="返回登录框，使用默认配置文件"
        >
          返回默认
        </button>
      </div>

      {/* ── 底部操作区 ── */}
      <div class="enter-spacer">
        {/* 确认进入按钮 */}
        <button class="btn-enter" onClick={confirm} disabled={loading}>
          {loading ? '验证中…' : '确认进入'}
        </button>
      </div>
    </div>
  );
}
