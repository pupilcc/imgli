import { describe, expect, it } from 'vitest'
import { cn } from './cn'

/**
 * tokens.css 里的自定义字号(text-2xs / text-xs-plus / text-sm-plus)不匹配
 * tailwind-merge 的 t-shirt 尺寸规则,未注册时会被当成文字颜色类,
 * 把同一次调用里的 text-muted / text-btn-text 吃掉 —— Segmented mono 选中项
 * 曾因此丢掉 text-btn-text,浅色深底深字、深色白底白字。
 */
describe('cn 保留自定义字号旁的颜色类', () => {
  it('text-muted 不被 text-xs-plus 吃掉', () => {
    const out = cn('text-muted', 'font-mono text-xs-plus').split(' ')
    expect(out).toContain('text-muted')
    expect(out).toContain('text-xs-plus')
  })

  it('text-btn-text 不被 text-xs-plus 吃掉', () => {
    const out = cn('bg-btn text-btn-text', 'font-mono text-xs-plus').split(' ')
    expect(out).toContain('text-btn-text')
    expect(out).toContain('text-xs-plus')
  })

  it('text-ink 与 text-sm-plus 共存', () => {
    const out = cn('text-ink', 'text-sm-plus').split(' ')
    expect(out).toContain('text-ink')
    expect(out).toContain('text-sm-plus')
  })

  it('text-muted 与 text-2xs 共存', () => {
    const out = cn('text-muted', 'text-2xs').split(' ')
    expect(out).toContain('text-muted')
    expect(out).toContain('text-2xs')
  })
})

describe('cn 冲突时后者胜', () => {
  it('自定义字号之间仍互斥', () => {
    expect(cn('text-xs-plus', 'text-2xs')).toBe('text-2xs')
  })

  it('自定义字号与内置字号互斥', () => {
    expect(cn('text-xs', 'text-xs-plus')).toBe('text-xs-plus')
    expect(cn('text-sm-plus', 'text-lg')).toBe('text-lg')
  })

  it('颜色类之间仍互斥', () => {
    expect(cn('text-muted', 'text-ink')).toBe('text-ink')
  })
})
