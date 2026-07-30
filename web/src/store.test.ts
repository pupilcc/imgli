import { act } from '@testing-library/react'
import { TOAST_MS, initialTheme, initialView, useGlobal } from './store'

beforeEach(() => {
  localStorage.clear()
  useGlobal.setState({ toasts: [], theme: 'light', view: 'masonry' })
  document.body.dataset.theme = ''
})

it('initialTheme 优先 localStorage，缺省 system', () => {
  expect(initialTheme()).toBe('system')
  localStorage.setItem('imgli-theme', 'dark')
  expect(initialTheme()).toBe('dark')
  localStorage.setItem('imgli-theme', 'light')
  expect(initialTheme()).toBe('light')
})

it('toggleTheme 循环 light→dark→system 并写 body dataset', () => {
  // beforeEach 把 store theme 置为 light
  act(() => useGlobal.getState().toggleTheme())
  expect(useGlobal.getState().theme).toBe('dark')
  expect(localStorage.getItem('imgli-theme')).toBe('dark')
  expect(document.body.dataset.theme).toBe('dark')

  act(() => useGlobal.getState().toggleTheme())
  expect(useGlobal.getState().theme).toBe('system')
  expect(localStorage.getItem('imgli-theme')).toBe('system')
  // resolveTheme(system) 走 matchMedia stub（matches:false）→ light
  expect(document.body.dataset.theme).toBe('light')
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
