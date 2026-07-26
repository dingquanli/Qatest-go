import { ref, computed, watch } from 'vue'

export type ThemeMode = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'qt_theme'

const theme = ref<ThemeMode>((localStorage.getItem(STORAGE_KEY) as ThemeMode) || 'light')

function systemPrefersDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

const isDark = computed<boolean>(() => {
  if (theme.value === 'system') return systemPrefersDark()
  return theme.value === 'dark'
})

function applyTheme(): void {
  const el = document.documentElement
  el.classList.remove('light', 'dark')
  el.classList.add(isDark.value ? 'dark' : 'light')
  el.style.colorScheme = isDark.value ? 'dark' : 'light'
}

applyTheme()

watch(isDark, applyTheme)

// 跟随系统模式下，监听系统主题变化
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
  if (theme.value === 'system') applyTheme()
})

export function useTheme() {
  function setTheme(t: ThemeMode): void {
    theme.value = t
    localStorage.setItem(STORAGE_KEY, t)
    applyTheme()
  }

  function toggle(): void {
    setTheme(isDark.value ? 'light' : 'dark')
  }

  return { theme, isDark, setTheme, toggle }
}
