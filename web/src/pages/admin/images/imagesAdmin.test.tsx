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
  created_at: '2026-07-17T10:00:00Z',
  policy_id: 1, policy_name: '本地', policy_driver: 'local', surface: 'public',
  path: 'public/2026/cat.png', in_trash: false, links, ...over,
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
      if (u.includes('/admin/images/') && method === 'POST' && u.includes('/restore'))
        return Promise.resolve(jsonRes(env({ key: 'k1', restored: true })))
      if (u.includes('/admin/images/') && method === 'DELETE') {
        const permanent = u.includes('permanent=1')
        return Promise.resolve(jsonRes(env({
          key: 'k1', deleted: true, permanent,
          physical_queued: permanent, object_retained: false,
        })))
      }
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

it('删除:两击确认软删 DELETE（无 permanent）', async () => {
  mockBackend([img()])
  renderPage()
  await screen.findByText('cat.png')
  await userEvent.click(screen.getByTitle('移入回收站'))
  await userEvent.click(screen.getByTitle('确认移入回收站'))
  await waitFor(() => {
    expect(lastReq?.method).toBe('DELETE')
    expect(lastReq?.url).toBe('/api/v1/admin/images/k1')
  })
})

it('列表:两击确认彻底删除 permanent=1', async () => {
  mockBackend([img()])
  renderPage()
  await screen.findByText('cat.png')
  await userEvent.click(screen.getByTitle('彻底删除'))
  await userEvent.click(screen.getByTitle('确认彻底删除（不可恢复）'))
  await waitFor(() => {
    expect(lastReq?.method).toBe('DELETE')
    expect(lastReq?.url).toContain('/api/v1/admin/images/k1?permanent=1')
  })
})

it('回收站:两击确认恢复 POST restore', async () => {
  mockBackend([img({ in_trash: true })])
  renderPage('/admin/images?deleted=trash')
  await screen.findByText('cat.png')
  await userEvent.click(screen.getByTitle('恢复'))
  await userEvent.click(screen.getByTitle('确认恢复'))
  await waitFor(() => {
    expect(lastReq?.method).toBe('POST')
    expect(lastReq?.url).toContain('/api/v1/admin/images/k1/restore')
  })
})

it('批量:勾选后彻底删除 POST batch purge', async () => {
  mockBackend([img(), img({ key: 'k2', name: 'dog.png' })])
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      const u = String(url)
      const method = init?.method ?? 'GET'
      if (method !== 'GET') lastReq = { url: u, method, body: init?.body ? JSON.parse(String(init.body)) : null }
      if (u.includes('/admin/policies'))
        return Promise.resolve(jsonRes(env({ items: [{ id: 1, name: '本地', driver: 'local', config: '{}', cdn_domain: '', path_template: '', enabled: true, created_at: '', file_count: 0, used_bytes: 0 }] })))
      if (u.includes('/admin/images/batch') && method === 'POST')
        return Promise.resolve(jsonRes(env({
          results: [
            { key: 'k1', ok: true, permanent: true, physical_queued: true },
            { key: 'k2', ok: true, permanent: true, physical_queued: true },
          ],
        })))
      if (u.includes('/admin/images'))
        return Promise.resolve(jsonRes(env({
          items: [img(), img({ key: 'k2', name: 'dog.png' })],
          total: 2, page: 1, limit: 50,
        })))
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
  renderPage()
  await screen.findByText('cat.png')
  await userEvent.click(screen.getByRole('button', { name: '全选本页' }))
  await userEvent.click(screen.getByRole('button', { name: '批量彻底删除' }))
  await userEvent.click(screen.getByRole('button', { name: '确认彻底删除（不可恢复）' }))
  await waitFor(() => {
    expect(lastReq?.method).toBe('POST')
    expect(lastReq?.url).toContain('/admin/images/batch')
    expect(lastReq?.body).toEqual({ keys: ['k1', 'k2'], action: 'purge' })
  })
})

it('详情:点卡片开 Modal 展示元信息与存储定位', async () => {
  mockBackend([img({ nsfw_score: 0.42 })])
  renderPage()
  await userEvent.click(await screen.findByText('cat.png'))
  const dialog = await screen.findByRole('dialog')
  expect(dialog).toBeInTheDocument()
  expect(within(dialog).getByText('0.42')).toBeInTheDocument()
  expect(within(dialog).getByText('2 KB')).toBeInTheDocument()
  expect(within(dialog).getByText(/本地 \(#1\)/)).toBeInTheDocument()
  expect(within(dialog).getByText('local')).toBeInTheDocument()
  expect(within(dialog).getByText('public/2026/cat.png')).toBeInTheDocument()
  expect(within(dialog).getByRole('button', { name: '移入回收站' })).toBeInTheDocument()
  expect(within(dialog).getByRole('button', { name: '彻底删除' })).toBeInTheDocument()
})

it('详情:彻底删除带 permanent=1', async () => {
  mockBackend([img()])
  renderPage()
  await userEvent.click(await screen.findByText('cat.png'))
  const dialog = await screen.findByRole('dialog')
  await userEvent.click(within(dialog).getByRole('button', { name: '彻底删除' }))
  await userEvent.click(within(dialog).getByRole('button', { name: '确认彻底删除（不可恢复）' }))
  await waitFor(() => {
    expect(lastReq?.method).toBe('DELETE')
    expect(lastReq?.url).toContain('/api/v1/admin/images/k1?permanent=1')
  })
})

it('详情:游客图仅彻底删除', async () => {
  mockBackend([img({ user_id: null, username: '' })])
  renderPage()
  await userEvent.click(await screen.findByText('cat.png'))
  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).getByText('游客')).toBeInTheDocument()
  expect(within(dialog).queryByRole('button', { name: '移入回收站' })).not.toBeInTheDocument()
  expect(within(dialog).getByRole('button', { name: '彻底删除' })).toBeInTheDocument()
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
