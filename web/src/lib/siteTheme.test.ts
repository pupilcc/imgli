import { describe, expect, it } from 'vitest'
import { contrastOnAccent, normalizeAccent, normalizeBgDim, normalizeGlass } from './siteTheme'

describe('siteTheme', () => {
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
})
