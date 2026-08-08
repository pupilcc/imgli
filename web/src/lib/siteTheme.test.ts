import { afterEach, describe, expect, it } from 'vitest'
import {
  applySiteTheme,
  contrastOnAccent,
  mixHex,
  normalizeAccent,
  normalizeBgDim,
  normalizeGlass,
  softSolidBgGradient,
} from './siteTheme'

describe('siteTheme', () => {
  afterEach(() => {
    applySiteTheme(null)
  })

  it('normalizes accent hex', () => {
    expect(normalizeAccent('')).toBe('')
    expect(normalizeAccent('#3B82F6')).toBe('#3b82f6')
    expect(normalizeAccent('#abc')).toBe('#aabbcc')
    expect(normalizeAccent('red')).toBe('')
  })

  it('picks contrast text for accent', () => {
    expect(contrastOnAccent('#000000')).toBe('#ffffff')
    expect(contrastOnAccent('#ffffff')).toBe('#17171a')
    expect(contrastOnAccent('#3b82f6')).toBe('#ffffff')
  })

  it('clamps bg dim', () => {
    expect(normalizeBgDim(undefined)).toBe(0.72)
    expect(normalizeBgDim(0.5)).toBe(0.5)
    expect(normalizeBgDim(-1)).toBe(0)
    expect(normalizeBgDim(2)).toBe(1)
  })

  it('clamps glass opacity', () => {
    expect(normalizeGlass(undefined)).toBe(0.78)
    expect(normalizeGlass(0.5)).toBe(0.5)
    expect(normalizeGlass(-1)).toBe(0)
    expect(normalizeGlass(2)).toBe(1)
  })

  it('mixHex and soft gradient', () => {
    expect(mixHex('#000000', '#ffffff', 0.5)).toBe('#808080')
    const g = softSolidBgGradient('#3b82f6')
    expect(g).toContain('linear-gradient')
    expect(g).toContain('#3b82f6')
  })

  it('applies solid bg as soft page wash (no image)', () => {
    applySiteTheme({ theme_bg_color: '#e8f0fe' })
    expect(document.body.dataset.bgColor).toBe('1')
    expect(document.body.style.getPropertyValue('--bg-solid')).toMatch(/^#/)
    expect(document.body.style.getPropertyValue('--bg-page')).toContain('linear-gradient')
    // with image: keep solid for scrim, drop page wash
    applySiteTheme({ theme_bg_color: '#e8f0fe', theme_bg_image_url: 'https://x/a.jpg' })
    expect(document.body.style.getPropertyValue('--bg-page')).toBe('')
    applySiteTheme({ theme_bg_color: '' })
    expect(document.body.style.getPropertyValue('--bg-solid')).toBe('')
    expect(document.body.dataset.bgColor).toBeUndefined()
  })
})
