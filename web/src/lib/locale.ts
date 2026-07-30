import type { Lang } from '../i18n/lang'

/** Operator copy: legacy plain string, or { zh, en } map. */
export type LocaleText = string | { zh?: string; en?: string } | null | undefined

/** Pick text for UI lang; missing side falls back to the other; empty → ''. */
export function pickLocale(raw: LocaleText, lang: Lang): string {
  if (raw == null) return ''
  if (typeof raw === 'string') return raw.trim()
  const zh = (raw.zh ?? '').trim()
  const en = (raw.en ?? '').trim()
  if (lang === 'zh') return zh || en
  return en || zh
}

/** True if any locale (or legacy string) has non-empty text. */
export function localeAny(raw: LocaleText): boolean {
  return pickLocale(raw, 'zh') !== '' || pickLocale(raw, 'en') !== ''
}

/** Stable fingerprint for dismiss / cache keys. */
export function localeFingerprint(raw: LocaleText): string {
  if (raw == null) return ''
  if (typeof raw === 'string') return raw
  return `${raw.zh ?? ''}\0${raw.en ?? ''}`
}

/** Normalize API value to always { zh, en } for admin forms. */
export function toLocaleMap(raw: LocaleText): { zh: string; en: string } {
  if (raw == null) return { zh: '', en: '' }
  if (typeof raw === 'string') return { zh: raw.trim(), en: '' }
  return { zh: (raw.zh ?? '').trim(), en: (raw.en ?? '').trim() }
}
