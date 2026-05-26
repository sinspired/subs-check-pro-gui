/**
 * frontend/src/components/ConfigSection.tsx
 */
import { JSX } from 'preact';
import { GuiApp } from '../../bindings/github.com/sinspired/subs-check-pro-gui';

interface Props {
  onSelect: (path: string) => void;
  toast:    (msg: string) => void;
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
    // OpenConfigFile 返回空字符串表示用户取消
    if (!path) return;
    onSelect(path);
  }

  return (
    <div id="configSection">
      <button class="btn-cfg" onClick={handleSelect}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none"
          stroke="currentColor" stroke-width="2">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
        </svg>
        选择其他配置文件…
      </button>
    </div>
  );
}
