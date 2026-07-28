import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import type { TrashItem } from '../../api/types'
import { TrashPage } from './TrashPage'

function jsonRes(body: unknown): Response {
  return { ok: true, status: 200, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })
const item = (k: string, days: number): TrashItem => ({
  key: k, name: `${k}.png`, ext: 'png', size: 2048, width: 10, height: 10,
  deleted_at: '2026-07-16T00:00:00Z', days_left: days,
})

class FakeIO {
  constructor(public cb: IntersectionObserverCallback) {}
  observe() {}
  disconnect() {}
  unobserve() {}
}

function mockBackend(items: TrashItem[]) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      const u = String(url)
      if (u.includes('/trash') && (!init || !init.method)) return Promise.resolve(jsonRes(env({ items, next_cursor: '' })))
      if (init?.method === 'POST') return Promise.resolve(jsonRes(env({ key: 'a', restored: true })))
      if (init?.method === 'DELETE') return Promise.resolve(jsonRes(env({ purged: items.length })))
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <TrashPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => vi.stubGlobal('IntersectionObserver', FakeIO as unknown as typeof IntersectionObserver))
afterEach(() => vi.unstubAllGlobals())

it('渲染卡片：剩余天数徽章（≤3 红）、恢复与彻底删除', async () => {
  mockBackend([item('a', 25), item('b', 2)])
  renderPage()
  expect(await screen.findByText('剩 25 天')).toBeInTheDocument()
  const urgent = screen.getByText('剩 2 天')
  expect(urgent.className).toMatch(/urgent/)
  expect(screen.getAllByRole('button', { name: '恢复' })).toHaveLength(2)
  expect(screen.getAllByRole('button', { name: '彻底删除' })).toHaveLength(2)
  expect(screen.getByText(/保留/)).toBeInTheDocument()
})

it('恢复走 POST restore', async () => {
  const user = userEvent.setup()
  mockBackend([item('a', 25)])
  renderPage()
  await screen.findByText('a.png')
  await user.click(screen.getAllByRole('button', { name: '恢复' })[0])
  await waitFor(() => {
    const f = vi.mocked(fetch)
    expect(f.mock.calls.some((c) => String(c[0]).includes('/trash/a/restore'))).toBe(true)
  })
})

it('彻底删除需两击；清空走对话框确认 DELETE /trash', async () => {
  const user = userEvent.setup()
  mockBackend([item('a', 25)])
  renderPage()
  await screen.findByText('a.png')
  await user.click(screen.getByRole('button', { name: '彻底删除' }))
  expect(vi.mocked(fetch).mock.calls.some((c) => (c[1] as RequestInit)?.method === 'DELETE')).toBe(false)
  await user.click(screen.getByRole('button', { name: '确认删除？' }))
  await waitFor(() => {
    expect(vi.mocked(fetch).mock.calls.some((c) => String(c[0]).includes('/trash/a') && (c[1] as RequestInit)?.method === 'DELETE')).toBe(true)
  })

  await user.click(screen.getByRole('button', { name: '清空回收站' }))
  expect(await screen.findByText('清空回收站？')).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '彻底删除全部' }))
  await waitFor(() => {
    expect(vi.mocked(fetch).mock.calls.some((c) => String(c[0]).endsWith('/api/v1/trash') && (c[1] as RequestInit)?.method === 'DELETE')).toBe(true)
  })
})

it('空态', async () => {
  mockBackend([])
  renderPage()
  expect(await screen.findByText('回收站是空的')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '清空回收站' })).not.toBeInTheDocument()
})
