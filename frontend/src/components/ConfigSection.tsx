/**
 * frontend/src/components/ConfigSection.tsx
 * 底部操作区：仅保留"选择其他配置文件"按钮（开机自启已移至左侧品牌区）
 */
import { JSX } from 'preact';
import { GuiApp } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

interface Props {
  onSelect:            (path: string) => void;
  toast:               (msg: string) => void;
  autostartEnabled:    boolean;
  onToggleAutostart:   (enabled: boolean) => void;
}

export function ConfigSection({ onSelect, toast }: Props): JSX.Element {

  async function handleSelect() {
    let path: string;
    try {
      path = await GuiApp.OpenConfigFile();
    } catch (e: any) {
      toast('打开文件对话框失败: ' + (e?.message ?? '未知错误'));
      return;
    }
    if (!path) return;
    onSelect(path);
  }

  return (
    <div class="cfg-bottom">
      <button class="btn-cfg-link" onClick={handleSelect} title="选择其他配置文件">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none"
          stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
        </svg>
        选择其他配置文件
      </button>
    </div>
  );
}
