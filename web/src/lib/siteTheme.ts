/** Apply operator site appearance (accent + background) from public/admin config. */

export type SiteThemeConfig = {
  theme_accent?: string | null
  /** Optional solid page background (#RGB/#RRGGBB); empty = theme default --bg-solid. */
  theme_bg_color?: string | null
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

function hexToRgb(hex: string): { r: number; g: number; b: number } | null {
  const a = normalizeAccent(hex)
  if (a.length !== 7) return null
  return {
    r: parseInt(a.slice(1, 3), 16),
    g: parseInt(a.slice(3, 5), 16),
    b: parseInt(a.slice(5, 7), 16),
  }
}

function rgbToHex(r: number, g: number, b: number): string {
  const c = (n: number) => Math.round(Math.min(255, Math.max(0, n))).toString(16).padStart(2, '0')
  return `#${c(r)}${c(g)}${c(b)}`
}

/** Mix two #RRGGBB colors; t=0 → a, t=1 → b. */
export function mixHex(a: string, b: string, t: number): string {
  const A = hexToRgb(a)
  const B = hexToRgb(b)
  if (!A || !B) return normalizeAccent(a) || '#000000'
  const u = Math.min(1, Math.max(0, t))
  return rgbToHex(A.r + (B.r - A.r) * u, A.g + (B.g - A.g) * u, A.b + (B.b - A.b) * u)
}

/** Near-white or near-black text for solid accent buttons. */
export function contrastOnAccent(hex: string): string {
  const rgb = hexToRgb(hex)
  if (!rgb) return '#ffffff'
  const luma = (0.2126 * rgb.r + 0.7152 * rgb.g + 0.0722 * rgb.b) / 255
  return luma > 0.55 ? '#17171a' : '#ffffff'
}

/**
 * Soft page wash from a solid brand color (avoids flat “poster” fill).
 * Mid stays near the chosen color; edges lightly lift/sink.
 */
export function softSolidBgGradient(hex: string): string {
  const mid = normalizeAccent(hex)
  if (!mid) return ''
  const top = mixHex(mid, '#ffffff', 0.16)
  const bot = mixHex(mid, '#000000', 0.12)
  // slight diagonal so it feels less like a UI flat fill
  return `linear-gradient(165deg, ${top} 0%, ${mid} 46%, ${bot} 100%)`
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

  // 纯色底：写入 --bg-solid；无图时用柔和渐变铺页面，避免纯色一块「生硬」
  const bgColor = normalizeAccent(cfg?.theme_bg_color)
  const bgURL = (cfg?.theme_bg_image_url ?? '').trim()
  if (bgColor) {
    // 略与中性灰混合，降低饱和冲击，仍可识别品牌色
    const solid = mixHex(bgColor, '#8a8a90', 0.08)
    body.style.setProperty('--bg-solid', solid)
    body.dataset.bgColor = '1'
    if (!bgURL) {
      body.style.setProperty('--bg-page', softSolidBgGradient(solid))
    } else {
      body.style.removeProperty('--bg-page')
    }
  } else {
    body.style.removeProperty('--bg-solid')
    body.style.removeProperty('--bg-page')
    delete body.dataset.bgColor
  }

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
