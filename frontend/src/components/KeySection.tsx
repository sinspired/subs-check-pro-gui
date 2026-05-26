/**
 * frontend/src/components/KeySection.tsx
 */
import { useState } from 'preact/hooks';
import { TransitionOverlay } from './TransitionOverlay';
import { GuiApp } from '../../bindings/github.com/sinspired/subs-check-pro-gui';
import { AppInfo } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

interface Props {
  info:  AppInfo;
  toast: (msg: string) => void;
}

export function KeySection({ info, toast }: Props) {
  const [keyShown, setKeyShown] = useState(false);
  // 随机 key 时默认勾选「记住密钥」；固定 key 默认不勾选
  const [remember, setRemember] = useState(info.keyIsRandom);

  const currentKey = info.apiKey;

  // 显示/隐藏
  const toggleKey = () => setKeyShown(v => !v);

  // 复制
  async function copyKey() {
    try {
      await navigator.clipboard.writeText(currentKey);
      toast('已复制密钥');
    } catch {
      toast('复制失败，请手动复制');
    }
  }

  const [launching, setLaunching] = useState(false);

  // 通过 nonce 进入 WebUI（apiKey 不出现在 URL）
  async function enterWebUI() {
    if (launching) return;
    setLaunching(true);

    // 先让过渡层完成一次绘制，避免尺寸调整时的拉伸/闪烁。
    await new Promise<void>(resolve => requestAnimationFrame(() => resolve()));

    let nonce: string;
    try {
      nonce = await GuiApp.GetEnterNonce(remember);
    } catch (e: any) {
      toast('获取登录凭证失败: ' + (e?.message ?? ''));
      setLaunching(false);
      return;
    }

    try {
      await GuiApp.ResizeToMain();
    } catch {
      // ResizeToMain 失败不阻断跳转
    }

    window.location.replace(
      `http://localhost:${info.listenPort}/gui/enter?n=${encodeURIComponent(nonce)}`
    );
  }

  return (
    <div id="keySection">
      {/* 首次运行 banner */}
      {info.isFirstRun && (
        <div class="hint first-run">
          🎉 首次运行 — 配置文件已创建：
          <code>{info.configPath || 'config/config.yaml'}</code>
        </div>
      )}

      {/* 随机 key 提示（非首次运行时显示） */}
      {info.keyIsRandom && !info.isFirstRun && (
        <div class="hint warn">
          ⚠️ 当前密钥随机生成，重启后将变更。建议在{' '}
          <code>config.yaml</code> 中固定 <code>api-key</code>。
        </div>
      )}

      {/* API Key 标签 */}
      <div class="label">API 密钥</div>

      {/* Key 展示行 */}
      <div class="key-wrap">
        <span
          class={`key-text${keyShown ? '' : ' blur'}`}
          onClick={toggleKey}
          title="点击显示/隐藏"
        >
          {currentKey}
        </span>

        {/* 显示/隐藏 */}
        <button class="icon-btn" onClick={toggleKey} title="显示/隐藏">
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

        {/* 复制 */}
        <button class="icon-btn" onClick={copyKey} title="复制密钥">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none"
            stroke="currentColor" stroke-width="2">
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
          </svg>
        </button>
      </div>

      {/* 端口信息 */}
      <div class="info-row">
        <div class="info-cell">
          <div class="lbl">HTTP 端口</div>
          <div class="val">{info.listenPort || '8199'}</div>
        </div>
        <div class="info-cell">
          <div class="lbl">Sub-Store</div>
          <div class="val">{info.subStorePort || '未启用'}</div>
        </div>
      </div>

      {/* 记住密钥 */}
      <label class="remember-row">
        <input
          type="checkbox"
          checked={remember}
          onChange={e => setRemember((e.target as HTMLInputElement).checked)}
        />
        <span>记住密钥（下次自动登录）</span>
      </label>

      {/* 进入按钮 */}
      <button class="btn-enter" onClick={enterWebUI} disabled={launching}>
        {launching ? '进入中…' : '进入管理界面 →'}
      </button>

      {launching && <TransitionOverlay message="正在进入管理界面…" />}
    </div>
  );
}
