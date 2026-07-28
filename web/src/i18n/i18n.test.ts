import { expect, test } from 'vitest'
import { detectLang, t } from './index'
import { useGlobal } from '../store'

test('t 取值中英', () => {
  useGlobal.setState({ lang: 'zh' })
  expect(t('common.save')).toBe('保存')
  useGlobal.setState({ lang: 'en' })
  expect(t('common.save')).toBe('Save')
})

test('t 插值', () => {
  useGlobal.setState({ lang: 'en' })
  expect(t('common.copied', { label: 'URL' })).toBe('Copied URL')
  useGlobal.setState({ lang: 'zh' })
  expect(t('common.copied', { label: 'URL' })).toBe('已复制 URL')
})

test('缺键回落 key', () => {
  expect(t('no.such.key')).toBe('no.such.key')
})

test('detectLang localStorage 优先', () => {
  localStorage.setItem('imgli-lang', 'en')
  expect(detectLang()).toBe('en')
  localStorage.setItem('imgli-lang', 'zh')
  expect(detectLang()).toBe('zh')
})
