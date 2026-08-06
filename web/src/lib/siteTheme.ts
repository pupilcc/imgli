/** Apply operator site appearance (accent + background) from public/admin config. */

export type SiteThemeConfig = {
  theme_accent?: string | null
  theme_bg_image_url?: string | null
  theme_bg_dim?: number | null
  /** Panel frosted opacity 0–1 when background image is set (default 0.78). */
  theme_glass?: number | null
}

const ACCENT_RE = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/

export function normalizeAccent(raw: string | null | undefined): string {
  const s = (raw ?? '').trim()
  if (!s || !ACCENT_RE.test(s)) return ''
  const lower = s.toLowerCase()
  if (lower.length === 4) {
    return `#${lower[1]}${lower[1]}${lower[2]}${lower[2]}${lower[3]}${lower[3]}`
  }
  return lower
}

/** Near-white or near-black text for solid accent buttons. */
export function contrastOnAccent(hex: string): string {
  const a = normalizeAccent(hex)
  if (a.length !== 7) return '#ffffff'
  const r = parseInt(a.slice(1, 3), 16)
  const g = parseInt(a.slice(3, 5), 16)
  const b = parseInt(a.slice(5, 7), 16)
  const luma = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255
  return luma > 0.55 ? '#17171a' : '#ffffff'
}

export function normalizeBgDim(raw: number | null | undefined): number {
  if (typeof raw !== 'number' || Number.isNaN(raw)) return 0.72
  if (raw < 0) return 0
  if (raw > 1) return 1
  return raw
}

export function normalizeGlass(raw: number | null | undefined): number {
  if (typeof raw !== 'number' || Number.isNaN(raw)) return 0.78
  if (raw < 0) return 0
  if (raw > 1) return 1
  return raw
}

function cssUrl(url: string): string {
  // Escape ) and quotes for CSS url("…")
  const safe = url.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\)/g, '\\)')
  return `url("${safe}")`
}

/** Write/clear body CSS variables for accent + optional full-page background. */
export function applySiteTheme(cfg: SiteThemeConfig | null | undefined): void {
  if (typeof document === 'undefined') return
  const body = document.body
  const accent = normalizeAccent(cfg?.theme_accent)
  if (accent) {
    body.style.setProperty('--btn', accent)
    body.style.setProperty('--btnText', contrastOnAccent(accent))
    body.style.setProperty('--accent', accent)
  } else {
    body.style.removeProperty('--btn')
    body.style.removeProperty('--btnText')
    body.style.removeProperty('--accent')
  }

  const bgURL = (cfg?.theme_bg_image_url ?? '').trim()
  if (bgURL) {
    body.dataset.bgImage = '1'
    body.style.setProperty('--bg-image', cssUrl(bgURL))
    body.style.setProperty('--bg-dim', String(normalizeBgDim(cfg?.theme_bg_dim)))
    body.style.setProperty('--glass', String(normalizeGlass(cfg?.theme_glass)))
  } else {
    delete body.dataset.bgImage
    body.style.removeProperty('--bg-image')
    body.style.removeProperty('--bg-dim')
    body.style.removeProperty('--glass')
  }
}
