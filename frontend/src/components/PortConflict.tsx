/**
 * frontend/src/components/PortConflict.tsx
 *
 * 端口冲突解决界面。
 * 修改端口 → SetPorts() 检查占用 → CompleteInit() 完成后端初始化 → onFixed()。
 */
import { useState } from 'preact/hooks';
import { GuiApp } from '../../bindings/github.com/sinspired/subs-check-pro-gui';
import { AppInfo } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

interface Props {
  info:    AppInfo;
  toast:   (msg: string) => void;
  onFixed: (newInfo: AppInfo) => void;
}

export function PortConflict({ info, toast, onFixed }: Props) {
  const [httpPort, setHttpPort]   = useState(info.listenPort || '8199');
  const [subPort,  setSubPort]    = useState(info.subStorePort || '');
  const [loading,  setLoading]    = useState(false);
  const showSub = !!(info.subStorePort && info.subStorePort !== '未启用');

  async function applyPorts() {
    setLoading(true);
    try {
      // 1. 校验并写入新端口到 GlobalConfig
      await GuiApp.SetPorts(httpPort.trim(), subPort.trim());
    } catch (e: any) {
      toast('❌ ' + (e?.message || '端口设置失败'));
      setLoading(false);
      return;
    }

    try {
      // 2. 若后端尚未初始化（pendingInit），完成初始化（启动 HTTP + sub-store）
      const currentInfo = await GuiApp.GetAppInfo();
      if (currentInfo.pendingInit) {
        await GuiApp.CompleteInit();
      }
    } catch (e: any) {
      toast('❌ 服务启动失败：' + (e?.message || '未知错误'));
      setLoading(false);
      return;
    }

    // 3. 重新拉取应用信息，确认端口冲突已解决
    let newInfo: AppInfo;
    try {
      newInfo = await GuiApp.GetAppInfo();
    } catch {
      toast('获取应用信息失败');
      setLoading(false);
      return;
    }

    if (newInfo.portConflictHTTP || newInfo.portConflictSubStore) {
      toast('端口仍被占用，请换一个');
      setHttpPort(newInfo.listenPort || '8199');
      setSubPort(newInfo.subStorePort || '');
      setLoading(false);
      return;
    }

    setLoading(false);
    onFixed(newInfo);
  }

  return (
    <div class="hint warn" id="portConflictBanner">
      <strong>⚠️ 端口冲突</strong> — 以下端口已被占用，请修改后继续：

      <div class="port-edit-row" style="margin-top:8px">
        {/* HTTP 端口 */}
        <div class="port-edit-cell">
          <label class="port-lbl">HTTP 端口</label>
          <input
            class="port-input"
            type="number"
            min="1024"
            max="65535"
            value={httpPort}
            disabled={loading}
            onInput={e => setHttpPort((e.target as HTMLInputElement).value)}
          />
        </div>

        {/* Sub-Store 端口（可选） */}
        {showSub && (
          <div class="port-edit-cell">
            <label class="port-lbl">Sub-Store 端口</label>
            <input
              class="port-input"
              type="number"
              min="1024"
              max="65535"
              value={subPort}
              disabled={loading}
              onInput={e => setSubPort((e.target as HTMLInputElement).value)}
            />
          </div>
        )}
      </div>

      <button class="btn-small" style="margin-top:8px" onClick={applyPorts} disabled={loading}>
        {loading ? '正在启动…' : '应用端口并启动服务'}
      </button>
    </div>
  );
}
