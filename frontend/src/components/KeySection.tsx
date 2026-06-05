/**
 * frontend/src/components/KeySection.tsx
 */
import { useState, useRef, useEffect } from 'preact/hooks';
import { GuiApp } from '../../bindings/github.com/sinspired/subs-check-pro-gui';
import { AppInfo } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

interface Props {
  info: AppInfo;
  toast: (msg: string) => void;
  onSelectConfig: (path: string) => void;
}

// ── 路径中间截断 Hook ───────────────────────────────────────────────────────
// 使用 canvas measureText 精确测量像素宽度，通过二分查找找到最大可显示字符数，
// 并在路径中间插入省略号，保留路径的头部（盘符/根目录）和尾部（文件名）。
// ResizeObserver 监听容器尺寸变化，自动重新计算。
function useTruncatedPath(path: string): {
  spanRef: ReturnType<typeof useRef<HTMLSpanElement | undefined>>;
  display: string;
} {
  const spanRef = useRef<HTMLSpanElement | undefined>(undefined);
  const [display, setDisplay] = useState(path);

  useEffect(() => {
    const el = spanRef.current;
    if (!el || !path) {
      setDisplay(path);
      return;
    }

    function compute() {
      const el2 = spanRef.current;
      if (!el2) return;

      // offsetWidth 是 flex 布局分配给该 span 的实际可用像素宽度
      const availW = el2.offsetWidth;
      if (availW <= 0) {
        setDisplay(path);
        return;
      }

      // 读取实际应用的字体属性，确保 canvas 测量与屏幕渲染一致
      const style = window.getComputedStyle(el2);
      const font = `${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;

      const canvas = document.createElement('canvas');
      const ctx = canvas.getContext('2d');
      if (!ctx) {
        setDisplay(path);
        return;
      }
      ctx.font = font;

      // 如果完整路径能放下，直接显示
      if (ctx.measureText(path).width <= availW) {
        setDisplay(path);
        return;
      }

      // 二分查找：找到最多可保留的总字符数 lo，使 front+ellipsis+back 恰好放得下
      // front = ceil(lo/2) 来自路径头部，back = floor(lo/2) 来自路径尾部（文件名侧）
      const ellipsis = '…';
      let lo = 0;
      let hi = path.length;

      while (hi - lo > 1) {
        const mid = (lo + hi) >> 1;
        const f = Math.ceil(mid / 2);
        const b = Math.floor(mid / 2);
        const candidate =
          path.slice(0, f) + ellipsis + (b > 0 ? path.slice(-b) : '');
        if (ctx.measureText(candidate).width <= availW) {
          lo = mid;
        } else {
          hi = mid;
        }
      }

      if (lo === 0) {
        // 连最短组合也放不下，只显示省略号
        setDisplay(ellipsis);
      } else {
        const f = Math.ceil(lo / 2);
        const b = Math.floor(lo / 2);
        setDisplay(
          path.slice(0, f) + ellipsis + (b > 0 ? path.slice(-b) : ''),
        );
      }
    }

    compute();

    // 容器宽度变化时重新计算（窗口缩放、布局变化等场景）
    const ro = new ResizeObserver(compute);
    ro.observe(el);
    return () => ro.disconnect();
  }, [path]);

  return { spanRef, display };
}

// ─────────────────────────────────────────────────────────────────────────────

export function KeySection({ info, toast, onSelectConfig }: Props) {
  const [keyShown, setKeyShown] = useState(false);
  const [launching, setLaunching] = useState(false);

  // 路径中间截断
  const { spanRef: pathRef, display: pathDisplay } = useTruncatedPath(
    info.configPath || '',
  );

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
    try {
      await GuiApp.EnterWebUI();

      // 延迟300ms重置状态
      setTimeout(() => {
        setLaunching(false);
      }, 300);
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
            {/* ref 绑定到 span 上，useTruncatedPath 通过 offsetWidth 获取可用宽度 */}
            <span class="cfg-path-text" ref={pathRef as any} title={info.configPath}>
              {pathDisplay}
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