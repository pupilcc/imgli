import { describe, expect, it } from 'vitest'
import { loginHref, safeNext } from './safeNext'

describe('safeNext', () => {
  it('defaults and rejects open redirect', () => {
    expect(safeNext(null)).toBe('/')
    expect(safeNext('')).toBe('/')
    expect(safeNext('https://evil.com')).toBe('/')
    expect(safeNext('//evil.com')).toBe('/')
    expect(safeNext('/login')).toBe('/')
    expect(safeNext('/login?x=1')).toBe('/')
  })

  it('allows in-app paths', () => {
    expect(safeNext('/')).toBe('/')
    expect(safeNext('/images')).toBe('/images')
    expect(safeNext('/albums/3?x=1')).toBe('/albums/3?x=1')
    expect(safeNext(encodeURIComponent('/settings'))).toBe('/settings')
  })
})

describe('loginHref', () => {
  it('encodes next=/', () => {
    expect(loginHref('/')).toBe('/login?next=%2F')
  })
})
