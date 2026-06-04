/**
 * frontend/src/components/KeySection.tsx
 */
import { useState } from 'preact/hooks';
import { GuiApp } from '../../bindings/github.com/sinspired/subs-check-pro-gui';
import { AppInfo } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

interface Props {
  info: AppInfo;
  toast: (msg: string) => void;
  onSelectConfig: (path: string) => void;
}

export function KeySection({ info, toast, onSelectConfig }: Props) {
  const [keyShown, setKeyShown] = useState(false);
  const [launching, setLaunching] = useState(false);

  const currentKey = info.apiKey;
  const toggleKey = () => setKeyShown(v => !v);

  async function copyKey() {
    try {
      await navigator.clipboard.writeText(currentKey);
      toast('已复制密钥');
    } catch {
      toast('复制失败，请手动复制');
    }
  }

  async function handleSelectConfig() {
    let path: string;
    try {
      path = await GuiApp.OpenConfigFile();
    } catch (e: any) {
      toast('打开文件对话框失败: ' + (e?.message ?? '未知错误'));
      return;
    }
    if (!path) return;
    onSelectConfig(path);
  }

  async function enterWebUI() {
    if (launching) return;
    setLaunching(true);
    let nonce: string;
    try {
      nonce = await GuiApp.GetEnterNonce(true);
    } catch (e: any) {
      toast('获取登录凭证失败: ' + (e?.message ?? ''));
      setLaunching(false);
      return;
    }
    const enterURL = `http://localhost:${info.listenPort}/gui/enter?n=${encodeURIComponent(nonce)}`;
    try {
      await GuiApp.EnterWebUI(enterURL);
    } catch (e: any) {
      toast('进入管理界面失败: ' + (e?.message ?? ''));
      setLaunching(false);
    }
  }

  return (
    <div id="keySection" class="key-section-flex">

      {/* ── key 主体区：flex:1 + justify-content:center 垂直居中 ── */}
      <div class="key-body">

        {info.isFirstRun && (
          <div class="hint first-run">
            🎉 首次运行 — 配置文件已创建：
            <code>{info.configPath || 'config/config.yaml'}</code>
          </div>
        )}

        {info.keyIsRandom && !info.isFirstRun && (
          <div class="hint warn">
            ⚠️ 当前密钥随机生成，重启后将变更。建议在{' '}
            <code>config.yaml</code> 中固定 <code>api-key</code>。
          </div>
        )}

        <div class="label">API 密钥</div>

        <div class="key-wrap">
          <span
            class={`key-text${keyShown ? '' : ' blur'}`}
            onClick={toggleKey}
            title="点击显示/隐藏"
          >
            {currentKey}
          </span>

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

          <button class="icon-btn" onClick={copyKey} title="复制密钥">
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none"
              stroke="currentColor" stroke-width="2">
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
              <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
            </svg>
          </button>
        </div>

        {info.configPath && (
          <div class="cfg-path-row">
            <svg class="cfg-path-icon" width="11" height="11" viewBox="0 0 24 24" fill="none"
              stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
              <polyline points="14 2 14 8 20 8" />
            </svg>
            <span class="cfg-path-text" title={info.configPath}>
              {info.configPath}
            </span>
            <button class="icon-btn cfg-path-btn" onClick={handleSelectConfig} title="选择其他配置文件">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none"
                stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
            </button>
          </div>
        )}
      </div>

      {/* ── 进入按钮：固定在底部，与版本栏保持固定间距 ── */}
      <div class="enter-spacer">
        <button class="btn-enter" onClick={enterWebUI} disabled={launching}>
          {launching ? '正在进入…' : '进入管理界面 →'}
        </button>
      </div>
    </div>
  );
}