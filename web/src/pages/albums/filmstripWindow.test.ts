import { describe, expect, it } from 'vitest'
import { filmstripWindow, shouldPrefetchMore } from './filmstripWindow'

describe('filmstripWindow', () => {
  it('空列表', () => {
    expect(filmstripWindow(0, 0)).toEqual({ start: 0, end: 0, padLeft: 0, padRight: 0 })
  })

  it('短列表全量', () => {
    expect(filmstripWindow(10, 3, 24, 62)).toEqual({ start: 0, end: 10, padLeft: 0, padRight: 0 })
  })

  it('长列表窗口与 spacer', () => {
    const w = filmstripWindow(100, 50, 10, 62)
    expect(w.start).toBe(40)
    expect(w.end).toBe(61)
    expect(w.padLeft).toBe(40 * 62)
    expect(w.padRight).toBe(39 * 62)
  })

  it('贴头/贴尾补满窗口', () => {
    const head = filmstripWindow(100, 2, 10, 62)
    expect(head.start).toBe(0)
    expect(head.end).toBe(21)
    const tail = filmstripWindow(100, 98, 10, 62)
    expect(tail.end).toBe(100)
    expect(tail.start).toBe(79)
  })
})

describe('shouldPrefetchMore', () => {
  it('末尾阈值内才预取', () => {
    expect(shouldPrefetchMore(10, 20, true, 6)).toBe(false)
    expect(shouldPrefetchMore(15, 20, true, 6)).toBe(true)
    expect(shouldPrefetchMore(19, 20, false, 6)).toBe(false)
  })
})
