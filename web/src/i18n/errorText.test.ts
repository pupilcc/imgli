import { expect, test } from 'vitest'
import { errorText } from './errorText'
import { useGlobal } from '../store'

test('en:已知 code → 英文译文', () => {
  useGlobal.setState({ lang: 'en' })
  expect(errorText('unauthorized', '请先登录')).toBe('Please sign in')
})

test('en:未知 code → 回落后端 message', () => {
  useGlobal.setState({ lang: 'en' })
  expect(errorText('weird_code', 'backend msg')).toBe('backend msg')
})

test('zh:恒用后端 message(保细分不丢信息,不回归)', () => {
  useGlobal.setState({ lang: 'zh' })
  expect(errorText('invalid_request', '不能封禁自己')).toBe('不能封禁自己')
  expect(errorText('unauthorized', '请先登录')).toBe('请先登录')
})

test('无 code → fallback', () => {
  useGlobal.setState({ lang: 'en' })
  expect(errorText(undefined, 'fb')).toBe('fb')
})
