import { useEffect, useState } from 'preact/hooks';

function getBaseURL(): string {
  return (window as any).__CORE_BASE_URL || 'http://127.0.0.1:8199';
}

function resolveTheme(t: string): string {
  if (t === 'auto' || !t) {
    return matchMedia('(prefers-color-scheme:dark)').matches ? 'dark' : 'light';
  }
  return t;
}

async function fetchThemeFromServer(): Promise<string> {
  try {
    const r = await fetch(`${getBaseURL()}/admin/theme`);
    const d = await r.json();
    if (d.theme && d.theme !== 'auto') return d.theme;
    // 服务端未记录明确值，读本地存储并回写到服务端
    const local = localStorage.getItem('scp_theme');
    if (local && local !== 'auto') {
      saveThemeToServer(local); // 让服务端记住，后续其他窗口也能读到
    }
    return resolveTheme(local || 'auto');
  } catch {
    return resolveTheme(localStorage.getItem('scp_theme') || 'auto');
  }
}

function saveThemeToServer(t: string) {
  fetch(`${getBaseURL()}/admin/theme`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ theme: t }),
  }).catch(() => { });
}

export function useTheme() {
  const [theme, setTheme] = useState<string>(resolveTheme('auto'));

  useEffect(() => {
    fetchThemeFromServer().then(setTheme);
  }, []);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
  }, [theme]);

  const toggleTheme = () => {
    const next = theme === 'dark' ? 'light' : 'dark';
    setTheme(next);
    saveThemeToServer(next);
  };

  const resetTheme = () => {
    const sys = resolveTheme('auto');
    setTheme(sys);
    saveThemeToServer('auto');
  };

  return { theme, toggleTheme, resetTheme };
}