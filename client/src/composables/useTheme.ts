import { ref } from 'vue';

type Theme = 'dark' | 'light';

const storageKey = 'opendownload-theme';

function initialTheme(): Theme {
  const saved = window.localStorage.getItem(storageKey);
  if (saved === 'dark' || saved === 'light') return saved;
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(value: Theme) {
  document.documentElement.classList.toggle('dark', value === 'dark');
  document.documentElement.classList.toggle('light', value === 'light');
  document.documentElement.style.colorScheme = value;
  window.localStorage.setItem(storageKey, value);

  const runtime = (window as Window & { runtime?: { WindowSetDarkTheme?: () => void; WindowSetLightTheme?: () => void } }).runtime;
  if (value === 'dark') runtime?.WindowSetDarkTheme?.();
  else runtime?.WindowSetLightTheme?.();
}

export function useTheme() {
  const theme = ref<Theme>(initialTheme());
  applyTheme(theme.value);
  const isDark = () => theme.value === 'dark';

  const toggleTheme = () => {
    theme.value = theme.value === 'dark' ? 'light' : 'dark';
    applyTheme(theme.value);
  };

  return { isDark, toggleTheme };
}
