import { describe, expect, it } from 'vitest'
import {
  adjustRgb,
  averageRgb,
  paletteFromImageData,
  rgbCss,
  sampleRegion,
} from './imagePalette'

describe('rgb helpers', () => {
  it('rgbCss / adjust / average', () => {
    expect(rgbCss({ r: 10, g: 20, b: 30 })).toBe('rgb(10, 20, 30)')
    expect(rgbCss({ r: 10, g: 20, b: 30 }, 0.5)).toBe('rgba(10, 20, 30, 0.5)')
    expect(adjustRgb({ r: 100, g: 100, b: 100 }, 0.5)).toEqual({ r: 50, g: 50, b: 50 })
    expect(averageRgb([])).toEqual({ r: 20, g: 20, b: 22 })
    expect(averageRgb([
      { r: 0, g: 0, b: 0 },
      { r: 100, g: 50, b: 0 },
    ])).toEqual({ r: 50, g: 25, b: 0 })
  })
})

describe('sampleRegion / paletteFromImageData', () => {
  it('纯色图采样得到该色', () => {
    const w = 10
    const h = 10
    const data = new Uint8ClampedArray(w * h * 4)
    for (let i = 0; i < w * h; i++) {
      data[i * 4] = 200
      data[i * 4 + 1] = 40
      data[i * 4 + 2] = 80
      data[i * 4 + 3] = 255
    }
    const mid = sampleRegion(data, w, h, 0, 0, w, h, 1)
    expect(mid.r).toBeCloseTo(200, 0)
    expect(mid.g).toBeCloseTo(40, 0)
    expect(mid.b).toBeCloseTo(80, 0)
    const p = paletteFromImageData(data, w, h)
    expect(p.top.r).toBeCloseTo(200, 0)
    expect(p.mid.b).toBeCloseTo(80, 0)
  })
})
