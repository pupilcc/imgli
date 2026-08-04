import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useNavigate } from 'react-router'
import { UsersPage } from './UsersPage'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })
const GB = 1024 ** 3

const groups = [
  { id: 1, name: '默认组', is_default: true, is_guest: false, storage_quota: 10 * GB, max_file_size: 0, bandwidth_quota_month: 5 * GB, rate_per_minute: 0, rate_per_hour: 0, rate_per_day: 0, allowed_exts: [], allowed_policy_ids: null, created_at: '', user_count: 2 },
  { id: 2, name: 'VIP', is_default: false, is_guest: false, storage_quota: 100 * GB, max_file_size: 0, bandwidth_quota_month: 50 * GB, rate_per_minute: 0, rate_per_hour: 0, rate_per_day: 0, allowed_exts: [], allowed_policy_ids: null, created_at: '', user_count: 0 },
]
const userRow = {
  id: 2,
  username: 'ling',
  email: 'ling@img.li',
  nickname: '凌',
  group_id: 1,
  status: 'active',
  is_admin: false,
  used_storage: 5 * GB,
  bandwidth_used_month: 1.5 * GB,
  bandwidth_period: '2026-08',
  created_at: '2026-07-16T00:00:00Z',
  last_seen_at: '2026-08-04T08:30:00Z',
  image_count: 12,
  email_verified: true,
}

let patched: Record<string, unknown> | null = null
function mockBackend() {
  patched = null
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      const u = String(url)
      if (u.endsWith('/auth/session'))
        return Promise.resolve(jsonRes(env({ id: 1, username: 'admin', email: 'a@img.li', nickname: '管', is_admin: true, created_at: '' })))
      if (u.includes('/admin/groups')) return Promise.resolve(jsonRes(env({ items: groups })))
      if (u.includes('/admin/users/2/reset-password'))
        return Promise.resolve(jsonRes(env({ password: 'ONE-TIME-PW' })))
      if (u.includes('/admin/users/2') && init?.method === 'PATCH') {
        patched = JSON.parse(String(init.body))
        return Promise.resolve(jsonRes(env({ ...userRow, ...patched })))
      }
      if (u.includes('/admin/users'))
        return Promise.resolve(jsonRes(env({ items: [userRow], total: 1, page: 1, limit: 50 })))
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
}

function renderPage(path = '/admin/users') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <UsersPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// 模拟「后退导航 / 外部跳转再返回」——不经由输入框打字,而是像浏览器历史那样直接改写 URL 的 q。
function NavHarness() {
  const navigate = useNavigate()
  return (
    <>
      <button onClick={() => navigate('/admin/users?q=ling')}>模拟后退到-q-ling</button>
      <UsersPage />
    </>
  )
}

function renderWithNav(path: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <NavHarness />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

it('表格:行数据/组名/容量/流量/状态', async () => {
  mockBackend()
  renderPage()
  expect(await screen.findByText('ling')).toBeInTheDocument()
  expect(screen.getByText('ling@img.li')).toBeInTheDocument()
  expect(screen.getByText('12')).toBeInTheDocument()
  // 容量 / 流量 两列都是 mono 数字（可能被 period 等同节点包住，用 getAll）
  const gb = screen.getAllByText(/GB/)
  expect(gb.some((el) => /5(\.0)? GB/.test(el.textContent || ''))).toBe(true)
  expect(gb.some((el) => /1\.5 GB/.test(el.textContent || ''))).toBe(true)
  expect(screen.getByText('正常')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '查看图片' })).toHaveAttribute('href', '/admin/images?user=2')
})

it('调组:行内 select PATCH group_id', async () => {
  mockBackend()
  renderPage()
  await screen.findByText('ling')
  await userEvent.selectOptions(screen.getByDisplayValue('默认组'), 'VIP')
  await waitFor(() => expect(patched).toEqual({ group_id: 2 }))
})

it('封禁:图标两击 PATCH status', async () => {
  mockBackend()
  renderPage()
  await screen.findByText('ling')
  await userEvent.click(screen.getByRole('button', { name: '封禁' }))
  await userEvent.click(screen.getByRole('button', { name: '确认封禁' }))
  await waitFor(() => expect(patched).toEqual({ status: 'banned' }))
})

it('重置密码:图标武装后开 Modal 再确认', async () => {
  mockBackend()
  renderPage()
  await screen.findByText('ling')
  await userEvent.click(screen.getByRole('button', { name: '重置密码' }))
  await userEvent.click(screen.getByRole('button', { name: '确认' }))
  await userEvent.click(await screen.findByRole('button', { name: '确认重置' }))
  expect(await screen.findByText('ONE-TIME-PW')).toBeInTheDocument()
  expect(screen.getByText(/仅显示一次/)).toBeInTheDocument()
})

it('表头排序:点流量写入 sort=bandwidth', async () => {
  mockBackend()
  renderPage()
  await screen.findByText('ling')
  await userEvent.click(screen.getByRole('button', { name: /按 本月流量 排序/ }))
  await waitFor(() => {
    const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls.map((c) => String(c[0]))
    expect(calls.some((u) => u.includes('sort=bandwidth'))).toBe(true)
  })
})

it('搜索:防抖后写入 q 参数请求', async () => {
  mockBackend()
  renderPage()
  await screen.findByText('ling')
  await userEvent.type(screen.getByPlaceholderText('搜索用户名 / 邮箱…'), 'abc')
  await waitFor(
    () => {
      const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls.map((c) => String(c[0]))
      expect(calls.some((u) => u.includes('/admin/users?q=abc'))).toBe(true)
    },
    { timeout: 2000 },
  )
})

it('搜索框初值随 URL 的 q 生效', async () => {
  mockBackend()
  renderPage('/admin/users?q=ling')
  await screen.findByText('ling')
  expect(screen.getByPlaceholderText('搜索用户名 / 邮箱…')).toHaveValue('ling')
})

it('后退导致 URL 的 q 外部变化:搜索框反向同步,防抖不会用旧输入把它覆盖回去', async () => {
  mockBackend()
  renderWithNav('/admin/users?q=abc')
  await screen.findByText('ling')
  expect(screen.getByPlaceholderText('搜索用户名 / 邮箱…')).toHaveValue('abc')

  await userEvent.click(screen.getByRole('button', { name: '模拟后退到-q-ling' }))
  await waitFor(() => expect(screen.getByPlaceholderText('搜索用户名 / 邮箱…')).toHaveValue('ling'))

  // 等过防抖窗口(300ms):若无反向同步,input 仍是旧值 'abc',防抖会把它推回 URL,
  // 请求会再次变回 q=abc——断言最后一次 /admin/users 请求仍停留在 q=ling,没有被吃回去。
  await new Promise((r) => setTimeout(r, 450))
  const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls
    .map((c) => String(c[0]))
    .filter((u) => u.includes('/admin/users?'))
  expect(calls.at(-1)).toContain('q=ling')
})
