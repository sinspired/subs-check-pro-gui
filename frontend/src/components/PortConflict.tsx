/**
 * frontend/src/components/PortConflict.tsx
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
  const [httpPort, setHttpPort] = useState(info.listenPort || '8199');
  const [subPort,  setSubPort]  = useState(info.subStorePort || '');
  const showSub = !!(info.subStorePort && info.subStorePort !== '未启用');

  async function applyPorts() {
    try {
      await GuiApp.SetPorts(httpPort.trim(), subPort.trim());
    } catch (e: any) {
      toast('❌ ' + (e?.message || '设置失败'));
      return;
    }

    let newInfo: AppInfo;
    try {
      newInfo = await GuiApp.GetAppInfo();
    } catch {
      toast('获取应用信息失败');
      return;
    }

    if (newInfo.portConflictHTTP || newInfo.portConflictSubStore) {
      toast('端口仍被占用，请换一个');
      // 更新本组件的输入框默认值
      setHttpPort(newInfo.listenPort || '8199');
      setSubPort(newInfo.subStorePort || '');
      return;
    }

    onFixed(newInfo);
  }

  return (
    <div class="hint warn" id="portConflictBanner">
      <strong>⚠️ 端口冲突</strong> — 以下端口已被占用，请修改后重新启动服务：

      <div class="port-edit-row" style="margin-top:8px">
        {/* HTTP 端口 */}
        <div class="port-edit-cell">
          <label class="port-lbl">HTTP 端口</label>
          <input
            class="port-input"
            type="number"
            min="1"
            max="65535"
            value={httpPort}
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
              min="1"
              max="65535"
              value={subPort}
              onInput={e => setSubPort((e.target as HTMLInputElement).value)}
            />
          </div>
        )}
      </div>

      <button class="btn-small" style="margin-top:8px" onClick={applyPorts}>
        应用端口并重试
      </button>
    </div>
  );
}
