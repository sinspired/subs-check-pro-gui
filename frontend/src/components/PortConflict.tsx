/**
 * frontend/src/components/PortConflict.tsx
 *
 * 端口冲突解决界面。
 *
 * 流程：
 *   1. 首次渲染 → 自动对初始端口调用 ValidatePort() 检测
 *   2. 输入变化 → debounce 300ms → ValidatePort()（格式 + 占用双检）
 *   3. 全部 ok → 按钮可点 → SetPorts()（内存 + YAML 持久化）→ CompleteInit()
 *   4. CompleteInit() 无异常即视为成功（backend 内部冲突标记不可信），直接 onFixed()
 *
 * 布局：
 *   - 单端口冲突 → 单列居中
 *   - 双端口均冲突 → 两列左右排列（pc-fields--row）
 */
import { useEffect, useRef, useState } from 'preact/hooks';
import { GuiApp } from '../../bindings/github.com/sinspired/subs-check-pro-gui';
import { AppInfo } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

interface Props {
  info: AppInfo;
  toast: (msg: string) => void;
  onFixed: (newInfo: AppInfo) => void;
}

type PortStatus = 'idle' | 'checking' | 'ok' | 'error';

interface FieldState {
  value: string;
  status: PortStatus;
  errMsg: string;
}

function initField(port: string): FieldState {
  return { value: port, status: 'idle', errMsg: '' };
}

// function StatusBadge({ status, msg }: { status: PortStatus; msg: string }) {
//   if (status === 'idle')     return null;
//   if (status === 'checking') return <span class="pc-status checking">检测中…</span>;
//   if (status === 'ok')       return <span class="pc-status ok">✓ 可用</span>;
//   return <span class="pc-status error" title={msg}>✗ {msg}</span>;
// }

export function PortConflict({ info, toast, onFixed }: Props) {
  // Sub-Store 端口冲突时才显示该字段
  const showSub = !!(
    info.subStorePort &&
    info.subStorePort !== '未启用' &&
    info.portConflictSubStore
  );

  const [http, setHttp] = useState<FieldState>(initField(info.listenPort || '8199'));
  const [sub, setSub] = useState<FieldState>(initField(info.subStorePort || ''));
  const [applying, setApplying] = useState(false);

  const httpTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const subTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const canApply =
    !applying &&
    http.status === 'ok' &&
    (!showSub || sub.status === 'ok');

  // ── 实时检测（debounce 300ms）────────────────────────────────
  function scheduleCheck(
    value: string,
    setField: (s: FieldState) => void,
    timerRef: { current: ReturnType<typeof setTimeout> | null },
  ) {
    if (timerRef.current) clearTimeout(timerRef.current);
    const v = value.trim();
    if (!v) { setField({ value, status: 'idle', errMsg: '' }); return; }

    setField({ value, status: 'checking', errMsg: '' });
    timerRef.current = setTimeout(async () => {
      try {
        const err = await GuiApp.ValidatePort(v);
        setField({ value, status: err ? 'error' : 'ok', errMsg: err ?? '' });
      } catch {
        setField({ value, status: 'error', errMsg: '检测失败，请重试' });
      }
    }, 300);
  }

  function onHttpInput(e: Event) {
    scheduleCheck((e.target as HTMLInputElement).value, setHttp, httpTimer);
  }
  function onSubInput(e: Event) {
    scheduleCheck((e.target as HTMLInputElement).value, setSub, subTimer);
  }

  // 首次渲染自动触发检测
  useEffect(() => {
    scheduleCheck(http.value, setHttp, httpTimer);
    if (showSub) scheduleCheck(sub.value, setSub, subTimer);
  }, []);

  // ── 应用端口并启动服务 ────────────────────────────────────────
  async function applyPorts() {
    setApplying(true);

    // Step 1: 更新端口（内存 + YAML 持久化）
    try {
      await GuiApp.SetPorts(http.value.trim(), showSub ? sub.value.trim() : '');
    } catch (e: any) {
      toast('❌ ' + (e?.message || '端口设置失败'));
      setApplying(false);
      return;
    }

    // Step 2: 若后端尚未初始化，完成初始化（启动 HTTP + Sub-Store）
    try {
      const cur = await GuiApp.GetAppInfo();
      if (cur.pendingInit) {
        await GuiApp.CompleteInit();
      }
    } catch (e: any) {
      toast('❌ 服务启动失败：' + (e?.message || '未知错误'));
      setApplying(false);
      return;
    }
    let newInfo: AppInfo;
    try {
      newInfo = await GuiApp.GetAppInfo();
    } catch {
      toast('获取应用信息失败');
      setApplying(false);
      return;
    }

    setApplying(false);
    onFixed(newInfo);
  }

  // ── 渲染 ─────────────────────────────────────────────────────
  return (
    <div class={`pc-root${showSub ? ' pc-root--wide' : ''}`}>

      {/* 标题区 */}
      <div class="pc-header">
        <div class="pc-icon">
          <svg width="26" height="26" viewBox="0 0 24 24" fill="none"
            stroke="currentColor" stroke-width="1.8"
            stroke-linecap="round" stroke-linejoin="round">
            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
            <line x1="12" y1="9" x2="12" y2="13" />
            <line x1="12" y1="17" x2="12.01" y2="17" />
          </svg>
        </div>
        <div>
          <h2 class="pc-title">端口冲突</h2>
          <p class="pc-subtitle">
            {showSub
              ? '以下两个端口均被占用，请各自修改后启动'
              : '以下端口已被其他程序占用，请修改后启动'}
          </p>
        </div>
      </div>

      {/* 端口字段区：双冲突时左右排列 */}
      <div class={`pc-fields${showSub ? ' pc-fields--row' : ''}`}>

        {/* HTTP 监听端口 */}
        <div class={`pc-field${http.status === 'error' ? ' has-error' : ''}${http.status === 'ok' ? ' has-ok' : ''}`}>
          <div class="pc-field-top">
            <label class="pc-label">
              HTTP
              {info.portConflictHTTP && <span class="pc-conflict-tag">冲突</span>}
            </label>
            {/* <StatusBadge status={http.status} msg={http.errMsg} /> */}
          </div>
          <input
            class="pc-input"
            type="number"
            min="1024" max="65535"
            value={http.value}
            disabled={applying}
            onInput={onHttpInput}
            placeholder="1024 – 65535"
          />
          {http.status === 'error' && <p class="pc-errmsg">{http.errMsg}</p>}
          <p class="pc-hint">订阅转换 Web 服务</p>
        </div>

        {/* Sub-Store 端口（仅在冲突时显示） */}
        {showSub && (
          <div class={`pc-field${sub.status === 'error' ? ' has-error' : ''}${sub.status === 'ok' ? ' has-ok' : ''}`}>
            <div class="pc-field-top">
              <label class="pc-label">
                Sub-Store
                {info.portConflictSubStore && <span class="pc-conflict-tag">冲突</span>}
              </label>
              {/* <StatusBadge status={sub.status} msg={sub.errMsg} /> */}
            </div>
            <input
              class="pc-input"
              type="number"
              min="1024" max="65535"
              value={sub.value}
              disabled={applying}
              onInput={onSubInput}
              placeholder="1024 – 65535"
            />
            {sub.status === 'error' && <p class="pc-errmsg">{sub.errMsg}</p>}
            <p class="pc-hint">Sub-Store 管理服务</p>
          </div>
        )}
      </div>

      {/* 摘要状态 */}
      <div class="pc-summary">
        {applying ? <span class="pc-summary-wait">正在启动服务，请稍候…</span>
          : canApply ? <span class="pc-summary-ok">✓ 端口可用，点击下方按钮启动</span>
            : <span class="pc-summary-hint">请修改端口直到全部显示「✓ 可用」</span>}
      </div>

      {/* 应用按钮 */}
      <button class="pc-apply-btn" onClick={applyPorts} disabled={!canApply}>
        {applying
          ? <><span class="spinner-sm" />正在启动…</>
          : '应用端口并启动服务'}
      </button>

    </div>
  );
}