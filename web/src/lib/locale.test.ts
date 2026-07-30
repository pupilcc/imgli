import { describe, expect, it } from 'vitest'
import { localeAny, pickLocale, toLocaleMap } from './locale'

describe('pickLocale', () => {
  it('legacy string', () => {
    expect(pickLocale('你好', 'zh')).toBe('你好')
    expect(pickLocale('你好', 'en')).toBe('你好')
  })
  it('map with fallback', () => {
    expect(pickLocale({ zh: '中', en: 'EN' }, 'zh')).toBe('中')
    expect(pickLocale({ zh: '中', en: 'EN' }, 'en')).toBe('EN')
    expect(pickLocale({ zh: '中', en: '' }, 'en')).toBe('中')
    expect(pickLocale({ zh: '', en: 'EN' }, 'zh')).toBe('EN')
  })
  it('empty', () => {
    expect(pickLocale(null, 'zh')).toBe('')
    expect(localeAny({ zh: '', en: '' })).toBe(false)
    expect(localeAny({ zh: 'x', en: '' })).toBe(true)
  })
  it('toLocaleMap', () => {
    expect(toLocaleMap('only')).toEqual({ zh: 'only', en: '' })
    expect(toLocaleMap({ zh: 'a', en: 'b' })).toEqual({ zh: 'a', en: 'b' })
  })
})
