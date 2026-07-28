import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { InvitesPage } from './InvitesPage'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

const ROWS = [
  { id: 1, code: 'IL-AAAA-2222', status: 'unused', created_by_name: 'boss', used_by_name: '', created_at: '2026-07-18T00:00:00Z', expires_at: null, used_at: null },
  { id: 2, code: 'IL-BBBB-3333', status: 'used', created_by_name: 'boss', used_by_name: 'ivy', created_at: '2026-07-18T00:00:00Z', expires_at: null, used_at: '2026-07-18T01:00:00Z' },
  { id: 3, code: 'IL-CCCC-4444', status: 'expired', created_by_name: 'boss', used_by_name: '', created_at: '2026-07-17T00:00:00Z', expires_at: '2026-07-18T00:00:00Z', used_at: null },
]

let created: unknown = null
let deleted: string | null = null
function mockBackend() {
  created = null
  deleted = null
  vi.stubGlobal('fetch', vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
    const u = String(url)
    if (u.includes('/admin/invites') && init?.method === 'POST') {
      created = JSON.parse(String(init.body))
      return Promise.resolve(jsonRes(env({ codes: ['IL-NEW1-2222', 'IL-NEW2-3333'] })))
    }
    if (u.includes('/admin/invites/') && init?.method === 'DELETE') {
      deleted = u
      return Promise.resolve(jsonRes(env({})))
    }
    if (u.includes('/admin/invites'))
      return Promise.resolve(jsonRes(env({ items: ROWS, total: 3, page: 1, limit: 50 })))
    return Promise.resolve(jsonRes(env(null)))
  }))
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/invites']}>
        <InvitesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

it('列表渲染码/状态/使用者', async () => {
  mockBackend()
  renderPage()
  expect(await screen.findByText('IL-AAAA-2222')).toBeInTheDocument()
  // 「未使用/已使用」同时存在于状态筛选 option 与行内 Tag
  expect(screen.getAllByText('未使用').length).toBeGreaterThanOrEqual(2)
  expect(screen.getAllByText('已使用').length).toBeGreaterThanOrEqual(2)
  expect(screen.getByText('ivy')).toBeInTheDocument()
})

it('生成:填数量提交,展示明文码列表', async () => {
  mockBackend()
  renderPage()
  const user = userEvent.setup()
  await screen.findByText('IL-AAAA-2222')
  await user.click(screen.getByRole('button', { name: '生成邀请码' }))
  const count = screen.getByLabelText('数量')
  await user.clear(count)
  await user.type(count, '2')
  await user.click(screen.getByRole('button', { name: '生成' }))
  // 明文码在同一 pre 内用换行拼接,用子串匹配
  expect(await screen.findByText(/IL-NEW1-2222/)).toBeInTheDocument()
  expect(screen.getByText(/IL-NEW2-3333/)).toBeInTheDocument()
  expect(created).toMatchObject({ count: 2 })
})

it('未用码两击撤销,已用码无撤销钮', async () => {
  mockBackend()
  renderPage()
  const user = userEvent.setup()
  await screen.findByText('IL-AAAA-2222')
  const btns = screen.getAllByRole('button', { name: '撤销' })
  expect(btns).toHaveLength(2)
  await user.click(btns[0])
  await user.click(screen.getByRole('button', { name: '确认撤销？' }))
  await waitFor(() => expect(deleted).toContain('/admin/invites/1'))
})

it('生成 pending 时关 Modal,resolve 后重开显示生成表单', async () => {
  let resolvePost!: (v: Response) => void
  const postP = new Promise<Response>((r) => {
    resolvePost = r
  })
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      const u = String(url)
      if (u.includes('/admin/invites') && init?.method === 'POST') return postP
      if (u.includes('/admin/invites'))
        return Promise.resolve(jsonRes(env({ items: ROWS, total: 3, page: 1, limit: 50 })))
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
  renderPage()
  const user = userEvent.setup()
  await screen.findByText('IL-AAAA-2222')
  await user.click(screen.getByRole('button', { name: '生成邀请码' }))
  await user.click(screen.getByRole('button', { name: '生成' }))
  await user.click(screen.getByRole('button', { name: '取消' }))
  resolvePost(jsonRes(env({ codes: ['IL-LATE-1111'] })))
  await waitFor(() => {})
  await user.click(screen.getByRole('button', { name: '生成邀请码' }))
  expect(screen.getByLabelText('数量')).toBeInTheDocument()
  expect(screen.queryByText(/IL-LATE-1111/)).not.toBeInTheDocument()
})

it('本页已清空且 total>0 显示返回第 1 页', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL) => {
      const u = String(url)
      if (u.includes('/admin/invites'))
        return Promise.resolve(jsonRes(env({ items: [], total: 5, page: 2, limit: 50 })))
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/invites?page=2']}>
        <InvitesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
  expect(await screen.findByText('本页已清空')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '返回第 1 页' })).toBeInTheDocument()
})
