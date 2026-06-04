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
    if (loading) return;

    setLoading(true);
    // 让 loading 状态先渲染出来
    await new Promise<void>(resolve => requestAnimationFrame(() => resolve()));

    let nonce: string;
    try {
      nonce = await GuiApp.ValidateConfigKey(trimmed, true);
    } catch (e: any) {
      toast('❌ ' + (e?.message || '密钥错误'));
      setLoading(false);
      return;
    }

    // 获取最新 AppInfo（用于构造 enterURL）
    let newInfo: AppInfo | null = null;
    try {
      newInfo = await GuiApp.GetAppInfo();
    } catch { /* 降级：使用默认端口 */ }

    const port     = newInfo?.listenPort || '8199';
    const enterURL = `http://localhost:${port}/gui/enter?n=${encodeURIComponent(nonce)}`;

    // 使用双窗口方案：Go 端在 webUIWin Navigate+Show，loginWin 由 Go 端隐藏。
    // loginWin 的 Wails 运行时全程保持活跃，关闭按钮不会失效。
    try {
      await GuiApp.EnterWebUI(enterURL);
    } catch (e: any) {
      toast('进入管理界面失败: ' + (e?.message ?? ''));
      setLoading(false);
    }
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
