/** 从图片采样主色，供沉浸页 letterbox 渐变底使用。 */

export type Rgb = { r: number; g: number; b: number }

export type ImagePalette = {
  top: Rgb
  bottom: Rgb
  left: Rgb
  right: Rgb
  mid: Rgb
}

export function rgbCss(c: Rgb, a = 1): string {
  const r = Math.round(c.r)
  const g = Math.round(c.g)
  const b = Math.round(c.b)
  if (a >= 1) return `rgb(${r}, ${g}, ${b})`
  return `rgba(${r}, ${g}, ${b}, ${a})`
}

/** 压暗/提亮，便于与主图衔接且不抢戏 */
export function adjustRgb(c: Rgb, factor: number, lift = 0): Rgb {
  return {
    r: Math.min(255, Math.max(0, c.r * factor + lift)),
    g: Math.min(255, Math.max(0, c.g * factor + lift)),
    b: Math.min(255, Math.max(0, c.b * factor + lift)),
  }
}

export function averageRgb(samples: Rgb[]): Rgb {
  if (!samples.length) return { r: 20, g: 20, b: 22 }
  let r = 0
  let g = 0
  let b = 0
  for (const s of samples) {
    r += s.r
    g += s.g
    b += s.b
  }
  const n = samples.length
  return { r: r / n, g: g / n, b: b / n }
}

/** 从 ImageData 取矩形区域平均色（step 采样） */
export function sampleRegion(
  data: Uint8ClampedArray,
  w: number,
  h: number,
  x0: number,
  y0: number,
  x1: number,
  y1: number,
  step = 2,
): Rgb {
  const samples: Rgb[] = []
  const left = Math.max(0, Math.floor(x0))
  const top = Math.max(0, Math.floor(y0))
  const right = Math.min(w, Math.ceil(x1))
  const bottom = Math.min(h, Math.ceil(y1))
  for (let y = top; y < bottom; y += step) {
    for (let x = left; x < right; x += step) {
      const i = (y * w + x) * 4
      const a = data[i + 3] ?? 0
      if (a < 16) continue
      samples.push({ r: data[i] ?? 0, g: data[i + 1] ?? 0, b: data[i + 2] ?? 0 })
    }
  }
  return averageRgb(samples)
}

export function paletteFromImageData(data: Uint8ClampedArray, w: number, h: number): ImagePalette {
  const band = Math.max(2, Math.floor(Math.min(w, h) * 0.12))
  const top = sampleRegion(data, w, h, 0, 0, w, band)
  const bottom = sampleRegion(data, w, h, 0, h - band, w, h)
  const left = sampleRegion(data, w, h, 0, 0, band, h)
  const right = sampleRegion(data, w, h, w - band, 0, w, h)
  const mid = sampleRegion(data, w, h, w * 0.2, h * 0.2, w * 0.8, h * 0.8, 3)
  return { top, bottom, left, right, mid }
}

/**
 * 加载图片并采样主色。优先同源缩略图；失败返回 null。
 */
export function extractImagePalette(src: string): Promise<ImagePalette | null> {
  return new Promise((resolve) => {
    if (!src || typeof document === 'undefined') {
      resolve(null)
      return
    }
    const img = new Image()
    img.decoding = 'async'
    try {
      img.crossOrigin = 'anonymous'
    } catch {
      /* ignore */
    }
    img.onload = () => {
      try {
        const maxSide = 64
        const nw = img.naturalWidth || img.width
        const nh = img.naturalHeight || img.height
        if (!nw || !nh) {
          resolve(null)
          return
        }
        const scale = Math.min(1, maxSide / Math.max(nw, nh))
        const w = Math.max(8, Math.round(nw * scale))
        const h = Math.max(8, Math.round(nh * scale))
        const canvas = document.createElement('canvas')
        canvas.width = w
        canvas.height = h
        const ctx = canvas.getContext('2d', { willReadFrequently: true })
        if (!ctx) {
          resolve(null)
          return
        }
        ctx.drawImage(img, 0, 0, w, h)
        const { data } = ctx.getImageData(0, 0, w, h)
        resolve(paletteFromImageData(data, w, h))
      } catch {
        resolve(null)
      }
    }
    img.onerror = () => resolve(null)
    img.src = src
  })
}

/** 四向边缘色 + 中心色 → CSS 多层渐变（letterbox 自然填色） */
export function paletteBackdropStyle(p: ImagePalette): {
  backgroundColor: string
  backgroundImage: string
} {
  const mid = adjustRgb(p.mid, 0.55, 8)
  const top = adjustRgb(p.top, 0.62, 6)
  const bottom = adjustRgb(p.bottom, 0.5, 4)
  const left = adjustRgb(p.left, 0.58, 6)
  const right = adjustRgb(p.right, 0.58, 6)
  const core = adjustRgb(p.mid, 0.35, 4)

  return {
    backgroundColor: rgbCss(core),
    backgroundImage: [
      `radial-gradient(ellipse 85% 75% at 50% 48%, ${rgbCss(mid, 0.45)} 0%, ${rgbCss(core, 0)} 72%)`,
      `linear-gradient(180deg, ${rgbCss(top)} 0%, ${rgbCss(mid, 0.88)} 42%, ${rgbCss(bottom)} 100%)`,
      `linear-gradient(90deg, ${rgbCss(left, 0.92)} 0%, ${rgbCss(mid, 0.12)} 45%, ${rgbCss(mid, 0.12)} 55%, ${rgbCss(right, 0.92)} 100%)`,
    ].join(', '),
  }
}
