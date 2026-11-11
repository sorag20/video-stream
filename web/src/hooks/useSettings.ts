import { useCallback, useEffect, useLayoutEffect, useState } from 'react'

export type Theme = 'system' | 'light' | 'dark'

export type Settings = {
  theme: Theme
  autoplay: boolean
}

const STORAGE_KEY = 'video-stream:settings'
const DEFAULTS: Settings = { theme: 'light', autoplay: false }
const THEMES: Theme[] = ['system', 'light', 'dark']

function loadSettings(): Settings {
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}') as Partial<Settings>
    return {
      theme: THEMES.includes(saved.theme as Theme) ? (saved.theme as Theme) : DEFAULTS.theme,
      autoplay: typeof saved.autoplay === 'boolean' ? saved.autoplay : DEFAULTS.autoplay,
    }
  } catch {
    // localStorage が使えない / 壊れた値が入っている場合は既定値で動かす
    return DEFAULTS
  }
}

// 設定はサーバーに保存せず、ブラウザ (localStorage) ごとに保持する
export function useSettings() {
  const [settings, setSettings] = useState(loadSettings)

  const updateSettings = useCallback((patch: Partial<Settings>) => {
    setSettings((prev) => ({ ...prev, ...patch }))
  }, [])

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(settings))
    } catch {
      // 保存できなくても、このセッション中は設定が有効
    }
  }, [settings])

  // 描画前に反映して、切り替え時のちらつきを防ぐ
  useLayoutEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const apply = () => {
      const resolved = settings.theme === 'system' ? (media.matches ? 'dark' : 'light') : settings.theme
      document.documentElement.dataset.theme = resolved
    }
    apply()
    if (settings.theme !== 'system') return
    media.addEventListener('change', apply)
    return () => media.removeEventListener('change', apply)
  }, [settings.theme])

  return { settings, updateSettings }
}
