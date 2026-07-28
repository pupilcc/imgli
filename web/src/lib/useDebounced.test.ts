import { act, renderHook } from '@testing-library/react'
import { useDebounced } from './useDebounced'

it('useDebounced 在静默 ms 后才更新', () => {
  vi.useFakeTimers()
  const { result, rerender } = renderHook(({ v }) => useDebounced(v, 300), { initialProps: { v: 'a' } })
  expect(result.current).toBe('a')
  rerender({ v: 'ab' })
  act(() => vi.advanceTimersByTime(200))
  expect(result.current).toBe('a')
  rerender({ v: 'abc' })
  act(() => vi.advanceTimersByTime(300))
  expect(result.current).toBe('abc')
  vi.useRealTimers()
})
