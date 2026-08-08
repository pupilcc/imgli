import { describe, expect, it } from 'vitest'
import { applyPattern, computeRename, previewBatchRename } from './batchRename'

describe('template tokens', () => {
  it('字面量 + 占位 + 转义 {{', () => {
    // {{ → 一个字面 {
    const out = applyPattern('pre_{name}_{n:02}_x{{y', {
      name: 'a',
      original: 'raw',
      ext: 'png',
      n1: 3,
      createdAt: '2026-08-08T12:00:00Z',
    })
    expect(out.startsWith('pre_a_03_x{')).toBe(true)
    expect(out.includes('{{')).toBe(false)
  })
  it('original / album', () => {
    expect(
      computeRename({
        name: 'raw_Brand_shot.jpg',
        ext: 'jpg',
        find: 'Brand',
        replace: '',
        ignoreCase: true,
        cleanSeparators: true,
        pattern: '{album}_{original}_{n}',
        n1: 1,
        album: '夏日',
      }),
    ).toBe('夏日_raw_Brand_shot_1.jpg')
  })
  it('{name} 是替换后的主名', () => {
    expect(
      computeRename({
        name: 'raw_Brand_shot.jpg',
        ext: 'jpg',
        find: 'Brand_',
        replace: '',
        ignoreCase: true,
        cleanSeparators: true,
        pattern: '{name}_{n:03}',
        n1: 2,
      }),
    ).toBe('raw_shot_002.jpg')
  })
})

describe('previewBatchRename', () => {
  it('未变更', () => {
    const rows = previewBatchRename([{ key: 'a', name: 'ok.png', ext: 'png' }], {
      find: 'zzz',
      replace: '',
      ignoreCase: true,
      cleanSeparators: true,
      pattern: '',
      startN: 1,
    })
    expect(rows[0]!.status).toBe('unchanged')
  })
})
