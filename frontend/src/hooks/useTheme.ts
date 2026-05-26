import { useEffect, useState } from 'preact/hooks';

const THEME_KEY = 'scp_theme';

export function useTheme() {
  const [theme, setTheme] = useState(
    localStorage.getItem(THEME_KEY) ||
    (matchMedia('(prefers-color-scheme:dark)').matches ? 'dark' : 'light')
  );

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem(THEME_KEY, theme);
  }, [theme]);

  const toggleTheme = () => {
    setTheme(t => (t === 'dark' ? 'light' : 'dark'));
  };

  return { theme, toggleTheme };
}
