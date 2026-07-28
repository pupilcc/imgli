import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { LogsPage } from './LogsPage'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

const rows = [
  { id: 3, actor_id: 1, actor_type: 'admin', action: 'review_approve', detail: '{"key":"T5B3aVum","score":0.92}', ip: '10.0.0.1', created_at: '2026-07-17T10:32:01Z' },
  { id: 2, actor_id: null, actor_type: 'system', action: 'moderation_flag', detail: '{"key":"abc","score":0.9}', ip: '', created_at: '2026-07-17T10:30:12Z' },
]

function mockBackend() {
  vi.stubGlobal('fetch', vi.fn((url: RequestInfo | URL) => {
    const u = String(url)
    if (u.includes('/admin/logs')) return Promise.resolve(jsonRes(env({ items: rows, total: rows.length, page: 1, limit: 50 })))
    return Promise.resolve(jsonRes(env(null)))
  }))
}

function renderPage(path = '/admin/logs') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <LogsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

// 注:动作中文标签(如「审核通过」「机审标记」)与操作/来源筛选下拉的 <option> 文案重复，
// 用 screen.findByText 做精确字符串匹配会在下拉与表格行两处命中同一文本，且在数据未加载完
// 时下拉的 <option> 已经先满足匹配、导致断言提前(错误地)通过。改用 getByRole('button', ...)
// 精确定位表格行(每行本身是一个 <button aria-expanded>)，规避与 <select><option> 的歧义。
it('表格:动作中文标签 + 操作者来源', async () => {
  mockBackend()
  renderPage()
  const row1 = await screen.findByRole('button', { name: /审核通过/ })
  expect(row1).toHaveTextContent('管理员')
  const row2 = screen.getByRole('button', { name: /机审标记/ })
  expect(row2).toHaveTextContent('系统')
})

it('点行内展开 detail 原文(格式化 JSON)', async () => {
  mockBackend()
  renderPage()
  const row = await screen.findByRole('button', { name: /审核通过/ })
  await userEvent.click(row)
  expect(await screen.findByText(/"score": 0\.92/)).toBeInTheDocument()
})

it('操作筛选写入 URL 并请求带 action', async () => {
  mockBackend()
  renderPage()
  await screen.findByRole('button', { name: /审核通过/ })
  await userEvent.selectOptions(screen.getByLabelText('操作筛选'), 'review_approve')
  await waitFor(() => {
    const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls.map((c) => String(c[0]))
    expect(calls.some((u) => u.includes('action=review_approve'))).toBe(true)
  })
})
