import { create } from 'zustand'
import { detectLang, setHtmlLang, type Lang } from './i18n/lang'

export type Theme = 'light' | 'dark' | 'system'
export type View = 'masonry' | 'grid' | 'list'
export type { Lang }
const THEME_KEY = 'imgli-theme'
const VIEW_KEY = 'imgli-view'
const LANG_KEY = 'imgli-lang'
export const TOAST_MS = 1600

export function initialTheme(): Theme {
  const saved = localStorage.getItem(THEME_KEY)
  if (saved === 'light' || saved === 'dark' || saved === 'system') return saved
  return 'system'
}

export function resolveTheme(t: Theme): 'light' | 'dark' {
  if (t === 'light' || t === 'dark') return t
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyResolved(t: Theme) {
  document.body.dataset.theme = resolveTheme(t)
}

export function initialView(): View {
  const v = localStorage.getItem(VIEW_KEY)
  return v === 'grid' || v === 'list' || v === 'masonry' ? v : 'masonry'
}

export interface Toast {
  id: number
  message: string
}

interface GlobalState {
  theme: Theme
  toggleTheme(): void
  toasts: Toast[]
  pushToast(message: string): void
  removeToast(id: number): void
  view: View
  setView(v: View): void
  lang: Lang
  setLang(l: Lang): void
}

let toastSeq = 0

export const useGlobal = create<GlobalState>((set, get) => ({
  theme: initialTheme(),
  toggleTheme() {
    const cur = get().theme
    const t: Theme = cur === 'light' ? 'dark' : cur === 'dark' ? 'system' : 'light'
    localStorage.setItem(THEME_KEY, t)
    applyResolved(t)
    set({ theme: t })
  },
  toasts: [],
  pushToast(message) {
    const id = ++toastSeq
    set((s) => ({ toasts: [...s.toasts, { id, message }] }))
    setTimeout(() => get().removeToast(id), TOAST_MS)
  },
  removeToast(id) {
    set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) }))
  },
  view: initialView(),
  setView(v) {
    localStorage.setItem(VIEW_KEY, v)
    set({ view: v })
  },
  lang: detectLang(),
  setLang(l) {
    localStorage.setItem(LANG_KEY, l)
    setHtmlLang(l)
    set({ lang: l })
  },
}))

/** 启动时把当前主题写到 body（此后由 toggleTheme 维护）。 */
export function applyTheme() {
  applyResolved(useGlobal.getState().theme)
  // system: follow OS changes
  try {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (useGlobal.getState().theme === 'system') applyResolved('system')
    })
  } catch { /* ignore */ }
}
