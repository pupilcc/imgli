import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { App } from '../../App'

vi.mock('react-chartjs-2', () => ({ Bar: () => <div data-testid="trend-chart" /> }))

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

function mockBackend(opts: { isAdmin: boolean; loggedIn?: boolean }) {
  const loggedIn = opts.loggedIn ?? true
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL) => {
      const u = String(url)
      if (u.endsWith('/auth/session'))
        return Promise.resolve(
          loggedIn
            ? jsonRes(env({ id: 1, username: 'root', email: 'r@img.li', nickname: '根', is_admin: opts.isAdmin, created_at: '' }))
            : jsonRes({ status: false, message: '未登录', data: { code: 'unauthorized' } }, 401),
        )
      if (u.includes('/admin/review'))
        return Promise.resolve(jsonRes(env({ items: [], total: 3, page: 1, limit: 1 })))
      if (u.endsWith('/admin/stats'))
        return Promise.resolve(jsonRes(env({ users: 2, images: 10, storage: 3 * 1024 ** 3, today_uploads: 1, daily: [] })))
      if (u.includes('/admin/logs'))
        return Promise.resolve(jsonRes(env({ items: [], total: 0, page: 1, limit: 8 })))
      if (u.endsWith('/user/quota'))
        return Promise.resolve(
          jsonRes(env({ used: 0, total: 1024 ** 3, max_file_size: 20 * 1024 ** 2, allowed_exts: ['png'] })),
        )
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
}

function renderAt(path: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

it('admin 访问 /admin:渲染仪表盘板块', async () => {
  mockBackend({ isAdmin: true })
  renderAt('/admin')
  expect(await screen.findByRole('heading', { name: '仪表盘' })).toBeInTheDocument()
})

it('未登录访问 /admin:跳转登录页', async () => {
  mockBackend({ isAdmin: false, loggedIn: false })
  renderAt('/admin')
  expect(await screen.findByTestId('auth-submit')).toBeInTheDocument()
})

it('非 admin 访问 /admin:重定向回前台上传页', async () => {
  mockBackend({ isAdmin: false })
  renderAt('/admin')
  expect(await screen.findByTestId('dropzone')).toBeInTheDocument()
  expect(screen.queryByRole('heading', { name: '仪表盘' })).not.toBeInTheDocument()
})

it('admin 访问 /admin/users:渲染用户管理页', async () => {
  mockBackend({ isAdmin: true })
  renderAt('/admin/users')
  expect(await screen.findByRole('heading', { name: '用户管理' })).toBeInTheDocument()
  expect(await screen.findByPlaceholderText('搜索用户名 / 邮箱…')).toBeInTheDocument()
})

it('admin 壳:侧栏九项 + 审核 badge + 返回前台', async () => {
  mockBackend({ isAdmin: true })
  renderAt('/admin')
  expect(await screen.findByRole('link', { name: /用户组/ })).toBeInTheDocument()
  expect(await screen.findByRole('link', { name: /邀请码/ })).toBeInTheDocument()
  expect(await screen.findByText('3')).toBeInTheDocument() // review badge(mock total=3)
  expect(screen.getByRole('link', { name: '← 返回前台' })).toHaveAttribute('href', '/')
  expect(screen.getByText('ADMIN')).toBeInTheDocument()
})

it('前台 Nav 头像菜单:admin 见「管理后台」,非 admin 不见', async () => {
  mockBackend({ isAdmin: true })
  renderAt('/')
  const { default: userEvent } = await import('@testing-library/user-event')
  await screen.findByTestId('dropzone')
  await userEvent.click(screen.getByText('根')) // 头像首字
  expect(await screen.findByRole('link', { name: '管理后台' })).toHaveAttribute('href', '/admin')
  await screen.findByText(/GB/)
})

it('非 admin 头像菜单无「管理后台」', async () => {
  mockBackend({ isAdmin: false })
  renderAt('/')
  const { default: userEvent } = await import('@testing-library/user-event')
  await screen.findByTestId('dropzone')
  await userEvent.click(screen.getByText('根'))
  expect(await screen.findByText('退出登录')).toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '管理后台' })).not.toBeInTheDocument()
  await screen.findByText(/GB/)
})

it('设置页:admin 见「管理后台 →」入口行', async () => {
  mockBackend({ isAdmin: true })
  renderAt('/settings')
  expect(await screen.findByRole('link', { name: /管理后台/ })).toHaveAttribute('href', '/admin')
})
