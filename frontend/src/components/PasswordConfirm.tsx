/**
 * frontend/src/components/PasswordConfirm.tsx
 */
import { useState } from 'preact/hooks';
import { GuiApp } from '../../bindings/github.com/sinspired/subs-check-pro-gui';
import { AppInfo } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

interface Props {
  cfgPath: string;
  toast:   (msg: string) => void;
  onDone:  (newInfo: AppInfo | null) => void;
}

export function PasswordConfirm({ cfgPath, toast, onDone }: Props) {
  const [key,     setKey]     = useState('');
  const [loading, setLoading] = useState(false);

  async function confirm() {
    const trimmed = key.trim();
    if (!trimmed) {
      toast('请输入 API 密钥');
      return;
    }

    setLoading(true);

    let nonce: string;
    try {
      nonce = await GuiApp.ValidateConfigKey(trimmed, true);
    } catch (e: any) {
      toast('❌ ' + (e?.message || '密钥错误'));
      setLoading(false);
      return;
    }

    // 调整窗口尺寸
    try { await GuiApp.ResizeToMain(); } catch { /* 可选 */ }

    // 获取最新 AppInfo
    let newInfo: AppInfo | null = null;
    try {
      newInfo = await GuiApp.GetAppInfo();
    } catch { /* 降级处理 */ }

    const port = newInfo?.listenPort || '8199';
    window.location.replace(
      `http://localhost:${port}/gui/enter?n=${encodeURIComponent(nonce)}`
    );

    onDone(newInfo);
  }

  return (
    <div class="password-confirm">
      <div class="label" style="margin-bottom:8px">
        请输入该配置文件的 API 密钥：
      </div>

      <div class="pw-row">
        <input
          class="pw-input"
          type="password"
          placeholder="API 密钥"
          value={key}
          autoFocus
          onInput={e => setKey((e.target as HTMLInputElement).value)}
          onKeyDown={e => { if (e.key === 'Enter') confirm(); }}
        />
        <button class="btn-small" onClick={confirm} disabled={loading}>
          {loading ? '验证中…' : '确认进入'}
        </button>
      </div>

      <div class="cfg-path-hint">{cfgPath}</div>
    </div>
  );
}
