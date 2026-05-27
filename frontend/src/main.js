/**
 * frontend/src/main.js — Wails 登录窗口前端逻辑
 *
 * Issue #1：非随机 key 模式下需主动选择"记住密钥"才能跨会话自动登录。
 * Issue #2：支持通过文件对话框选择自定义配置文件，需输入密钥验证。
 * Issue #3：端口冲突时展示可编辑端口字段。
 * Issue #5：通过一次性 nonce 替代明文 apiKey 在 URL 中传输。
 * Issue #6：提供托盘最小化相关 JS 侧逻辑（通知已由 Go 侧处理）。
 */

// ── 主题 ──────────────────────────────────────────────────────────────────────

const THEME_KEY = 'scp_theme';

function applyTheme(t) {
  document.documentElement.setAttribute('data-theme', t);
  const isDark = t === 'dark';
  document.getElementById('iDark').style.display  = isDark ? 'none' : '';
  document.getElementById('iLight').style.display = isDark ? '' : 'none';
}

applyTheme(
  localStorage.getItem(THEME_KEY) ||
  (matchMedia('(prefers-color-scheme:dark)').matches ? 'dark' : 'light')
);

document.getElementById('themeBtn').addEventListener('click', () => {
  const next = document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
  localStorage.setItem(THEME_KEY, next);
  applyTheme(next);
});

// ── Toast ─────────────────────────────────────────────────────────────────────

let _toastTimer;
function toast(msg, duration = 2200) {
  const el = document.getElementById('toast');
  el.textContent = msg;
  el.classList.add('show');
  clearTimeout(_toastTimer);
  _toastTimer = setTimeout(() => el.classList.remove('show'), duration);
}

// ── Wails 就绪等待 ────────────────────────────────────────────────────────────

function waitForWails() {
  return new Promise((resolve, reject) => {
    if (window.go?.frontend?.GuiApp) { resolve(); return; }
    let elapsed = 0;
    const timer = setInterval(() => {
      elapsed += 50;
      if (window.go?.frontend?.GuiApp) { clearInterval(timer); resolve(); }
      else if (elapsed > 5000) { clearInterval(timer); reject(new Error('Wails bindings not available')); }
    }, 50);
  });
}

// ── Key 显示/隐藏 ─────────────────────────────────────────────────────────────

let keyShown = false;
let currentKey = '';

function syncKeyVisibility() {
  const el = document.getElementById('keyText');
  el.textContent = currentKey;
  el.classList.toggle('blur', !keyShown);
  document.getElementById('iEyeOff').style.display = keyShown ? 'none' : '';
  document.getElementById('iEyeOn').style.display  = keyShown ? '' : 'none';
}

function toggleKey() { keyShown = !keyShown; syncKeyVisibility(); }

// ── 主初始化 ──────────────────────────────────────────────────────────────────

async function initLoginPage() {
  try {
    await waitForWails();
    const info = await window.go.frontend.GuiApp.GetAppInfo();

    document.getElementById('loadingState').style.display = 'none';

    // Issue #1：首次运行 banner（精简）
    if (info.isFirstRun) {
      const banner = document.getElementById('firstRunBanner');
      if (banner) {
        const cpEl = document.getElementById('configPathVal');
        if (cpEl) cpEl.textContent = info.configPath || 'config/config.yaml';
        banner.style.display = '';
      }
    }

    // Issue #3：端口冲突
    if (info.portConflictHTTP || info.portConflictSubStore) {
      showPortConflict(info);
      return; // 先让用户处理端口再继续
    }

    renderKeySection(info);

  } catch (err) {
    document.getElementById('loadingState').innerHTML =
      `<span style="color:var(--warn)">⚠️ 初始化失败: ${err.message}</span>`;
    console.error('GetAppInfo failed:', err);
  }
}

// ── 渲染 Key 区域 ─────────────────────────────────────────────────────────────

function renderKeySection(info) {
  currentKey = info.apiKey;

  // Issue #1：非随机 key 时，默认不勾选"记住密钥"
  const rememberChk = document.getElementById('rememberChk');
  rememberChk.checked = !!info.keyIsRandom;

  if (info.keyIsRandom && !info.isFirstRun) {
    document.getElementById('randomKeyHint').style.display = '';
  }

  document.getElementById('portVal').textContent     = info.listenPort   || '8199';
  document.getElementById('subStoreVal').textContent = info.subStorePort || '未启用';

  syncKeyVisibility();
  document.getElementById('keySection').style.display   = '';
  document.getElementById('configSection').style.display = '';

  document.getElementById('revealBtn').addEventListener('click', toggleKey);
  document.getElementById('keyText').addEventListener('click', toggleKey);
  document.getElementById('copyBtn').addEventListener('click', () => copyKey(info.apiKey));

  // Issue #5：进入时获取 nonce，而非直接传 apiKey
  document.getElementById('enterBtn').addEventListener('click', () =>
    enterWebUI(info.listenPort)
  );

  // Issue #2：选择配置文件按钮
  document.getElementById('selectCfgBtn').addEventListener('click', selectConfigFile);
}

// ── Issue #5：通过 nonce 进入 WebUI（apiKey 不出现在 URL） ────────────────────

async function enterWebUI(port) {
  try {
    await window.go.frontend.GuiApp.ResizeToMain();
  } catch (e) {
    console.warn('ResizeToMain failed:', e);
  }

  const remember = document.getElementById('rememberChk').checked;

  let nonce;
  try {
    nonce = await window.go.frontend.GuiApp.GetEnterNonce(remember);
  } catch (e) {
    toast('获取登录凭证失败: ' + e.message);
    return;
  }

  // nonce 替代明文 apiKey，浏览器历史中不再含密钥
  window.location.replace(
    `http://localhost:${port}/gui/enter?n=${encodeURIComponent(nonce)}`
  );
}

// ── Issue #2：选择自定义配置文件 ─────────────────────────────────────────────

async function selectConfigFile() {
  let path;
  try {
    path = await window.go.frontend.GuiApp.OpenConfigFile();
  } catch (e) {
    toast('打开文件对话框失败: ' + e.message);
    return;
  }
  if (!path) return; // 用户取消

  // 暂时通过 ValidateConfigKey("", false) 来触发配置重载
  // 实际上我们只需要显示密码输入框让用户验证
  showPasswordConfirm(path);
}

function showPasswordConfirm(configPath) {
  document.getElementById('keySection').style.display    = 'none';
  document.getElementById('configSection').style.display = 'none';

  const pc = document.getElementById('passwordConfirm');
  const hint = document.getElementById('cfgPathHint');
  hint.textContent = configPath;
  pc.style.display = '';

  const input = document.getElementById('cfgKeyInput');
  input.value = '';
  input.focus();

  document.getElementById('confirmKeyBtn').onclick = () => confirmConfigKey(configPath);
  input.onkeydown = e => { if (e.key === 'Enter') confirmConfigKey(configPath); };
}

async function confirmConfigKey(configPath) {
  const key = document.getElementById('cfgKeyInput').value.trim();
  if (!key) { toast('请输入 API 密钥'); return; }

  const remember = document.getElementById('rememberChk').checked;

  let nonce;
  try {
    nonce = await window.go.frontend.GuiApp.ValidateConfigKey(key, remember);
  } catch (e) {
    toast('❌ ' + (e.message || '密钥错误'));
    document.getElementById('cfgKeyInput').select();
    return;
  }

  // 验证通过，调整窗口并跳转
  try { await window.go.frontend.GuiApp.ResizeToMain(); } catch {}

  const port = document.getElementById('portVal').textContent || '8199';
  window.location.replace(
    `http://localhost:${port}/gui/enter?n=${encodeURIComponent(nonce)}`
  );
}

// ── Issue #3：端口冲突处理 ────────────────────────────────────────────────────

function showPortConflict(info) {
  const banner = document.getElementById('portConflictBanner');
  banner.style.display = '';

  const httpInput = document.getElementById('httpPortInput');
  const subCell   = document.getElementById('subStorePortCell');
  const subInput  = document.getElementById('subStorePortInput');

  httpInput.value = info.listenPort || '8199';

  if (info.subStorePort && info.subStorePort !== '未启用') {
    subCell.style.display = '';
    subInput.value = info.subStorePort;
  }

  document.getElementById('applyPortBtn').addEventListener('click', async () => {
    const http = httpInput.value.trim();
    const sub  = subInput.value.trim();
    try {
      await window.go.frontend.GuiApp.SetPorts(http, sub);
      // 端口更新成功，重新获取信息并渲染
      banner.style.display = 'none';
      const newInfo = await window.go.frontend.GuiApp.GetAppInfo();
      if (!newInfo.portConflictHTTP && !newInfo.portConflictSubStore) {
        renderKeySection(newInfo);
      } else {
        toast('端口仍被占用，请换一个');
        showPortConflict(newInfo);
      }
    } catch (e) {
      toast('❌ ' + (e.message || '设置失败'));
    }
  });
}

// ── 工具函数 ──────────────────────────────────────────────────────────────────

async function copyKey(key) {
  try {
    await navigator.clipboard.writeText(key);
    const btn = document.getElementById('copyBtn');
    btn.classList.add('ok');
    toast('已复制密钥');
    setTimeout(() => btn.classList.remove('ok'), 1400);
  } catch {
    toast('复制失败，请手动复制');
  }
}

// ── 启动 ──────────────────────────────────────────────────────────────────────
window.addEventListener('DOMContentLoaded', initLoginPage);
