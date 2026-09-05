import { computed, ref } from 'vue';
import { THEME_NAMES, THEME_STORAGE_KEY } from './constants';

type Theme = (typeof THEME_NAMES)[number];

function initialTheme(): Theme {
  const saved = window.localStorage.getItem(THEME_STORAGE_KEY);
  if (saved === 'dark' || saved === 'light') {
    return saved;
  }

  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(value: Theme): void {
  document.documentElement.classList.toggle('dark', value === 'dark');
  document.documentElement.classList.toggle('light', value === 'light');
  document.documentElement.style.colorScheme = value;
  window.localStorage.setItem(THEME_STORAGE_KEY, value);

  const runtime = (window as Window & {
    runtime?: {
      WindowSetDarkTheme?: () => void;
      WindowSetLightTheme?: () => void;
    };
  }).runtime;

  if (value === 'dark') {
    runtime?.WindowSetDarkTheme?.();
    return;
  }

  runtime?.WindowSetLightTheme?.();
}

const theme = ref<Theme>(initialTheme());
applyTheme(theme.value);

export function useTheme() {
  const isDark = computed(() => theme.value === 'dark');

  function toggleTheme(): void {
    theme.value = theme.value === 'dark' ? 'light' : 'dark';
    applyTheme(theme.value);
  }

  return { isDark, toggleTheme };
}
