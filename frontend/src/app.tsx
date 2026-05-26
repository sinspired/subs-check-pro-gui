/**
 * frontend/src/app.tsx
 * 登录窗口根组件 — Wails3 + Preact + TypeScript
 */
import { useEffect, useState } from 'preact/hooks';

import { useTheme }     from './hooks/useTheme';
import { useToast }     from './hooks/useToast';
import { useWailsReady } from './hooks/useWailsReady';

import { Header }          from './components/Header';
import { KeySection }      from './components/KeySection';
import { ConfigSection }   from './components/ConfigSection';
import { PortConflict }    from './components/PortConflict';
import { PasswordConfirm } from './components/PasswordConfirm';
import { Toast }           from './components/Toast';

import { GuiApp }  from '../bindings/github.com/sinspired/subs-check-pro-gui';
import { AppInfo } from '../bindings/github.com/sinspired/subs-check-pro-gui';

// UI 状态机：每个状态对应一个独立视图
type View = 'loading' | 'error' | 'portConflict' | 'main' | 'password';

export function App() {
  const ready                           = useWailsReady();
  const { theme, toggleTheme }          = useTheme();
  const { msg, visible, toast }         = useToast();

  const [view, setView]     = useState<View>('loading');
  const [info, setInfo]     = useState<AppInfo | null>(null);
  const [errMsg, setErrMsg] = useState('');
  const [cfgPath, setCfgPath] = useState('');

  // Wails 就绪后立即拉取应用信息
  useEffect(() => {
    if (!ready) return;
    loadAppInfo();
  }, [ready]);

  async function loadAppInfo() {
    setView('loading');
    try {
      const data = await GuiApp.GetAppInfo();
      setInfo(data);
      if (data.portConflictHTTP || data.portConflictSubStore) {
        setView('portConflict');
      } else {
        setView('main');
      }
    } catch (e: any) {
      setErrMsg(e?.message ?? '未知错误');
      setView('error');
    }
  }

  // 端口冲突解决后更新 info 并切换视图
  function handlePortsFixed(newInfo: AppInfo) {
    setInfo(newInfo);
    setView('main');
  }

  // 选择配置文件 → 进入密码确认
  function handleSelectConfig(path: string) {
    setCfgPath(path);
    setView('password');
  }

  // 密码确认完成后（已跳转，这里做降级回退）
  function handlePasswordDone(newInfo: AppInfo | null) {
    if (newInfo) setInfo(newInfo);
    setView('main');
  }

  return (
    <>
      {view === 'loading' && (
        <div class="page">
          <div class="card">
            <Header theme={theme} toggleTheme={toggleTheme} />
            <div class="state-box">
              <div class="spinner" />
              <span>正在加载应用信息…</span>
            </div>
          </div>
        </div>
      )}

      {view === 'error' && (
        <div class="page">
          <div class="card">
            <Header theme={theme} toggleTheme={toggleTheme} />
            <div class="state-box" style="color:var(--warn)">
              ⚠️ 初始化失败：{errMsg}
            </div>
          </div>
        </div>
      )}

      {view === 'portConflict' && info && (
        <div class="page">
          <div class="card">
            <Header theme={theme} toggleTheme={toggleTheme} />
            <PortConflict
              info={info}
              toast={toast}
              onFixed={handlePortsFixed}
            />
          </div>
        </div>
      )}

      {view === 'main' && info && (
        <div class="page">
          <div class="card">
            <Header theme={theme} toggleTheme={toggleTheme} />
            <KeySection info={info} toast={toast} />
            <ConfigSection onSelect={handleSelectConfig} toast={toast} />
          </div>
        </div>
      )}

      {view === 'password' && (
        <div class="page">
          <div class="card">
            <Header theme={theme} toggleTheme={toggleTheme} />
            <PasswordConfirm
              cfgPath={cfgPath}
              toast={toast}
              onDone={handlePasswordDone}
            />
          </div>
        </div>
      )}

      <Toast msg={msg} visible={visible} />
    </>
  );
}
