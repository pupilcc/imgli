import { useGlobal } from '../store'
import zh from './locales/zh'
import en from './locales/en'
import { type Lang, detectLang, setHtmlLang } from './lang'

export type { Lang }
export { detectLang, setHtmlLang }

const dicts = { zh, en }

// 点路径取值 + {var} 插值;缺键 console.warn(dev) 回落 key
export function t(key: string, vars?: Record<string, string | number>): string {
  const lang = useGlobal.getState().lang
  const raw = key.split('.').reduce<any>((o, k) => (o == null ? o : o[k]), dicts[lang])
  if (typeof raw !== 'string') {
    if (import.meta.env?.DEV) console.warn('[i18n] missing key:', key, lang)
    return key
  }
  return vars ? raw.replace(/\{(\w+)\}/g, (_, k) => String(vars[k] ?? `{${k}}`)) : raw
}

export function useT(): { t: typeof t; lang: Lang; setLang: (l: Lang) => void } {
  const lang = useGlobal((s) => s.lang)
  const setLang = useGlobal((s) => s.setLang)
  return { t, lang, setLang }
}
