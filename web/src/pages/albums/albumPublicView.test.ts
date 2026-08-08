import { describe, expect, it } from 'vitest'
import { buildAlbumSearch, parseIndexParam, parseViewParam } from './albumPublicView'

describe('parseIndexParam', () => {
  it('1-based → 0-based；非法回落 0', () => {
    expect(parseIndexParam(null)).toBe(0)
    expect(parseIndexParam('')).toBe(0)
    expect(parseIndexParam('1')).toBe(0)
    expect(parseIndexParam('3')).toBe(2)
    expect(parseIndexParam('0')).toBe(0)
    expect(parseIndexParam('x')).toBe(0)
  })
})

describe('buildAlbumSearch', () => {
  it('gallery 空串；immersive 带 view+i', () => {
    expect(buildAlbumSearch('gallery', 0)).toBe('')
    expect(buildAlbumSearch('immersive', 0)).toBe('view=immersive&i=1')
    expect(buildAlbumSearch('immersive', 4)).toBe('view=immersive&i=5')
  })
})

describe('parseViewParam', () => {
  it('仅 immersive 进入沉浸', () => {
    expect(parseViewParam(null)).toBe('gallery')
    expect(parseViewParam('gallery')).toBe('gallery')
    expect(parseViewParam('immersive')).toBe('immersive')
    expect(parseViewParam('other')).toBe('gallery')
  })
})
