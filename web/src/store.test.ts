import { act } from '@testing-library/react'
import { TOAST_MS, initialTheme, initialView, useGlobal } from './store'

beforeEach(() => {
  localStorage.clear()
  useGlobal.setState({ toasts: [], theme: 'light', view: 'masonry' })
  document.body.dataset.theme = ''
})

it('initialTheme 优先 localStorage，其次系统偏好', () => {
  expect(initialTheme()).toBe('light') // matchMedia stub matches:false
  localStorage.setItem('imgli-theme', 'dark')
  expect(initialTheme()).toBe('dark')
})

it('toggleTheme 持久化并写 body dataset', () => {
  act(() => useGlobal.getState().toggleTheme())
  expect(useGlobal.getState().theme).toBe('dark')
  expect(localStorage.getItem('imgli-theme')).toBe('dark')
  expect(document.body.dataset.theme).toBe('dark')
})

it('pushToast 1.6s 后自动移除', () => {
  vi.useFakeTimers()
  act(() => useGlobal.getState().pushToast('已复制 URL 链接'))
  expect(useGlobal.getState().toasts).toHaveLength(1)
  act(() => vi.advanceTimersByTime(TOAST_MS + 10))
  expect(useGlobal.getState().toasts).toHaveLength(0)
  vi.useRealTimers()
})

it('view 偏好持久化且非法值回退', () => {
  useGlobal.getState().setView('list')
  expect(localStorage.getItem('imgli-view')).toBe('list')
  expect(useGlobal.getState().view).toBe('list')

  localStorage.setItem('imgli-view', 'bogus')
  expect(initialView()).toBe('masonry')
})
