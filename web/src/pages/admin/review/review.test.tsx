import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { useGlobal } from '../../../store'
import { ReviewPage } from './ReviewPage'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

const item = (key: string, score: number | null) => ({
  key, name: `${key}.png`, ext: 'png', size: 2048, visibility: 'public', status: 'pending',
  is_whitelisted: false, nsfw_score: score, username: 'ling', user_id: 2,
  created_at: '2026-07-16T00:00:00Z',
  links: { url: `/i/${key}`, thumbnail_url: `/t/${key}`, delete_url: '', markdown: '', bbcode: '' },
})

let decided: unknown = null
let batched: unknown = null
function mockBackend(items: unknown[], total = items.length, batchResults: unknown[] = []) {
  decided = null
  batched = null
  vi.stubGlobal('fetch', vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
    const u = String(url)
    if (u.includes('/admin/review/batch')) {
      batched = JSON.parse(String(init!.body))
      return Promise.resolve(jsonRes(env({ results: batchResults })))
    }
    if (init?.method === 'POST' && u.includes('/admin/review/')) {
      decided = { url: u, body: JSON.parse(String(init.body)) }
      return Promise.resolve(jsonRes(env({ key: 'k1', status: 'normal' })))
    }
    if (u.includes('/admin/review'))
      return Promise.resolve(jsonRes(env({ items, total, page: 1, limit: 50 })))
    return Promise.resolve(jsonRes(env(null)))
  }))
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/review']}>
        <ReviewPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

it('空队列:ALL CLEAR 空态', async () => {
  mockBackend([])
  renderPage()
  expect(await screen.findByText('ALL CLEAR')).toBeInTheDocument()
})

it('本页已清空但其他页仍有待审:不误报 ALL CLEAR,提供返回第 1 页', async () => {
  mockBackend([], 60)
  renderPage()
  expect(await screen.findByText('本页已清空')).toBeInTheDocument()
  expect(screen.queryByText('ALL CLEAR')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '返回第 1 页' })).toBeInTheDocument()
})

it('卡片流:图/上传者/NSFW,通过单击 POST approve', async () => {
  mockBackend([item('k1', 0.91)])
  renderPage()
  expect(await screen.findByText('k1.png')).toBeInTheDocument()
  expect(screen.getByText(/ling/)).toBeInTheDocument()
  expect(screen.getByText(/NSFW 0\.91/)).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '通过' }))
  await waitFor(() => expect(decided).toEqual({ url: expect.stringContaining('/admin/review/k1'), body: { action: 'approve' } }))
})

it('展示机审 triggers 插件与分数', async () => {
  const withTrig = {
    ...item('k1', 0.82),
    triggers: [{ plugin: 'nsfwjs', severity: 'review', score: 0.82 }],
  }
  mockBackend([withTrig])
  renderPage()
  // chip 文案: nsfwjs · review · 0.82
  expect(await screen.findByText(/nsfwjs · review · 0\.82/)).toBeInTheDocument()
})

it('拒绝单击 POST reject(无二次确认)', async () => {
  mockBackend([item('k1', 0.2)])
  renderPage()
  await screen.findByText('k1.png')
  await userEvent.click(screen.getByRole('button', { name: '拒绝' }))
  await waitFor(() => expect((decided as { body: unknown }).body).toEqual({ action: 'reject' }))
})

it('全部通过:batch 当前页 keys + approve', async () => {
  mockBackend([item('k1', 0.2), item('k2', 0.3)])
  renderPage()
  await screen.findByText('k1.png')
  await userEvent.click(screen.getByRole('button', { name: /全部通过/ }))
  await waitFor(() => expect(batched).toEqual({ keys: ['k1', 'k2'], action: 'approve' }))
})

it('批量部分失败:计数 toast', async () => {
  mockBackend([item('k1', 0.2), item('k2', 0.3)], 2, [
    { key: 'k1', ok: true },
    { key: 'k2', ok: false, error: 'x' },
  ])
  renderPage()
  await screen.findByText('k1.png')
  await userEvent.click(screen.getByRole('button', { name: /全部通过/ }))
  await waitFor(() =>
    expect(useGlobal.getState().toasts.some((t) => /1 张通过，1 张失败/.test(t.message))).toBe(true),
  )
})

it('批量全部成功:不弹 toast', async () => {
  useGlobal.setState({ toasts: [] })
  mockBackend([item('k1', 0.2), item('k2', 0.3)], 2, [
    { key: 'k1', ok: true },
    { key: 'k2', ok: true },
  ])
  renderPage()
  await screen.findByText('k1.png')
  await userEvent.click(screen.getByRole('button', { name: /全部通过/ }))
  await waitFor(() => expect(batched).toEqual({ keys: ['k1', 'k2'], action: 'approve' }))
  expect(useGlobal.getState().toasts).toEqual([])
})
