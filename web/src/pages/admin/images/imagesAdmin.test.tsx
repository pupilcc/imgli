import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { ImagesAdminPage } from './ImagesAdminPage'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

const links = {
  url: 'http://x/i/k1.png', markdown: '', html: '', bbcode: '', thumbnail_url: 'http://x/t/k1.png',
}
const img = (over: Record<string, unknown> = {}) => ({
  key: 'k1', name: 'cat.png', ext: 'png', size: 2048, visibility: 'public', status: 'normal',
  is_whitelisted: false, nsfw_score: null, username: 'ling', user_id: 2,
  created_at: '2026-07-17T10:00:00Z', links, ...over,
})

let lastReq: { url: string; method: string; body: unknown } | null = null
function mockBackend(items: unknown[], opts: { pageEmptiedTotal?: number } = {}) {
  lastReq = null
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      const u = String(url)
      const method = init?.method ?? 'GET'
      if (method !== 'GET') lastReq = { url: u, method, body: init?.body ? JSON.parse(String(init.body)) : null }
      if (u.includes('/admin/policies'))
        return Promise.resolve(jsonRes(env({ items: [{ id: 1, name: '本地', driver: 'local', config: '{}', cdn_domain: '', path_template: '', enabled: true, created_at: '', file_count: 0, used_bytes: 0 }] })))
      if (u.includes('/admin/images/') && method === 'PATCH')
        return Promise.resolve(jsonRes(env(img({ is_whitelisted: true }))))
      if (u.includes('/admin/images/') && method === 'DELETE')
        return Promise.resolve(jsonRes(env({ key: 'k1', deleted: true })))
      if (u.includes('/admin/images') && opts.pageEmptiedTotal != null && u.includes('page=2'))
        return Promise.resolve(jsonRes(env({ items: [], total: opts.pageEmptiedTotal, page: 2, limit: 50 })))
      if (u.includes('/admin/images'))
        return Promise.resolve(jsonRes(env({ items, total: items.length, page: 1, limit: 50 })))
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
}

function renderPage(path = '/admin/images') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <ImagesAdminPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

it('网格:卡片、上传者、状态徽章', async () => {
  mockBackend([img(), img({ key: 'k2', name: 'dog.png', status: 'pending', is_whitelisted: true })])
  renderPage()
  expect(await screen.findByText('cat.png')).toBeInTheDocument()
  expect(screen.getAllByText('ling')).toHaveLength(2)
  // 「待审」同时存在于状态筛选 option 与卡片徽章:2 处
  expect(screen.getAllByText('待审')).toHaveLength(2)
  expect(screen.getByText('WL')).toBeInTheDocument()
})

it('user 筛选 chip:显示并可清除', async () => {
  mockBackend([img()])
  renderPage('/admin/images?user=2')
  expect(await screen.findByText('用户 #2')).toBeInTheDocument()
  const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls.map((c) => String(c[0]))
  expect(calls.some((u) => u.includes('user=2'))).toBe(true)
  await userEvent.click(screen.getByRole('button', { name: '清除用户筛选' }))
  await waitFor(() => {
    const after = (fetch as ReturnType<typeof vi.fn>).mock.calls.map((c) => String(c[0]))
    expect(after.some((u) => u.includes('/admin/images') && !u.includes('user='))).toBe(true)
  })
})

it('加白:hover 按钮两击确认后 PATCH is_whitelisted', async () => {
  mockBackend([img()])
  renderPage()
  await screen.findByText('cat.png')
  await userEvent.click(screen.getByTitle('加白'))
  await userEvent.click(screen.getByTitle('确认加白'))
  await waitFor(() => expect(lastReq).toEqual({ url: '/api/v1/admin/images/k1', method: 'PATCH', body: { is_whitelisted: true } }))
})

it('删除:两击确认 DELETE', async () => {
  mockBackend([img()])
  renderPage()
  await screen.findByText('cat.png')
  await userEvent.click(screen.getByTitle('删除'))
  await userEvent.click(screen.getByTitle('确认删除'))
  await waitFor(() => expect(lastReq?.method).toBe('DELETE'))
})

it('详情:点卡片开 Modal 展示元信息', async () => {
  mockBackend([img({ nsfw_score: 0.42 })])
  renderPage()
  await userEvent.click(await screen.findByText('cat.png'))
  const dialog = await screen.findByRole('dialog')
  expect(dialog).toBeInTheDocument()
  // 网格卡片仍挂载在 Modal 之下,「2 KB」在卡片 meta 与详情 dd 中各出现一次,故限定在弹窗内断言
  expect(within(dialog).getByText('0.42')).toBeInTheDocument()
  expect(within(dialog).getByText('2 KB')).toBeInTheDocument()
})

it('空态', async () => {
  mockBackend([])
  renderPage()
  expect(await screen.findByText('没有匹配的图片')).toBeInTheDocument()
})

it('page>1 本页已清空:与筛选空态区分,可返回第 1 页', async () => {
  mockBackend([], { pageEmptiedTotal: 60 })
  renderPage('/admin/images?page=2')
  expect(await screen.findByText('本页已清空')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '返回第 1 页' })).toBeInTheDocument()
  expect(screen.queryByText('没有匹配的图片')).not.toBeInTheDocument()
})
