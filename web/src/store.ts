import { create } from 'zustand'
import { detectLang, setHtmlLang, type Lang } from './i18n/lang'

export type Theme = 'light' | 'dark'
export type View = 'masonry' | 'grid' | 'list'
export type { Lang }
const THEME_KEY = 'imgli-theme'
const VIEW_KEY = 'imgli-view'
const LANG_KEY = 'imgli-lang'
export const TOAST_MS = 1600

export function initialTheme(): Theme {
  const saved = localStorage.getItem(THEME_KEY)
  if (saved === 'light' || saved === 'dark') return saved
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
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
    const t: Theme = get().theme === 'light' ? 'dark' : 'light'
    localStorage.setItem(THEME_KEY, t)
    document.body.dataset.theme = t
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
  document.body.dataset.theme = useGlobal.getState().theme
}
