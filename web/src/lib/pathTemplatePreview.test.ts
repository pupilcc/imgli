import { describe, expect, it } from 'vitest'
import { previewObjectKey, previewPathTemplate } from './pathTemplatePreview'

describe('pathTemplatePreview', () => {
  const now = new Date(2026, 7, 8, 15, 4, 5, 123) // local Aug 8

  it('renders default-like template', () => {
    const p = previewPathTemplate('{Y}/{m}/{d}/{uniqid}.{ext}', 'png', now)
    expect(p).toMatch(/^2026\/08\/08\/[0-9A-Za-z]{12}\.png$/)
  })

  it('builds object key with prefix + public surface', () => {
    const k = previewObjectKey({
      prefix: 'upload',
      template: '{uniqid}.{ext}',
      ext: 'png',
    })
    expect(k.startsWith('upload/public/')).toBe(true)
    expect(k.endsWith('.png')).toBe(true)
  })
})
