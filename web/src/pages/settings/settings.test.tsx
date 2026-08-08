import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { createQueryClient } from '../../queryClient'
import { useGlobal } from '../../store'
import { SettingsPage } from './SettingsPage'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })
const GB = 1024 ** 3

const EMPTY_PREFS = {
  default_album_id: null as number | null,
  default_visibility: '' as const,
  default_policy_id: null as number | null,
  auto_copy_format: '' as const,
  watermark: { enabled: false, position: '', opacity: 0, margin: 0 },
}

const SESSION_USER = {
  id: 1,
  username: 'isian',
  email: 'i@img.li',
  nickname: '凌',
  is_admin: true,
  email_verified: false,
  created_at: '',
  preferences: EMPTY_PREFS,
  avatar_url: '',
  watermark_set: false,
  public_profile: false,
}

const ALBUM_A = {
  id: 11,
  name: '相册 A',
  visibility: 'public',
  image_count: 0,
  cover_key: '',
  created_at: '',
}
const ALBUM_B = {
  id: 12,
  name: '相册 B',
  visibility: 'public',
  image_count: 0,
  cover_key: '',
  created_at: '',
}

let tokenCreated = false
function mockBackend(
  sessionUser: typeof SESSION_USER = SESSION_USER,
  opts: { policies?: { id: number; name: string }[]; plazaEnabled?: boolean } = {},
) {
  const policies = opts.policies ?? [{ id: 1, name: '本地' }]
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      const u = String(url)
      if (u.endsWith('/auth/session')) return Promise.resolve(jsonRes(env(sessionUser)))
      if (u.endsWith('/config'))
        return Promise.resolve(
          jsonRes(
            env({
              site_name: 'img.li',
              registration_mode: 'open',
              guest_upload_enabled: false,
              plaza_enabled: !!opts.plazaEnabled,
              guest: null,
              base_url: 'https://img.li',
            }),
          ),
        )
      if (u.endsWith('/auth/resend-verification') && init?.method === 'POST') return Promise.resolve(jsonRes(env(null)))
      if (u.endsWith('/user/quota'))
        return Promise.resolve(jsonRes(env({ used: 2.14 * GB, total: 10 * GB, max_file_size: 20 * 1024 ** 2, allowed_exts: ['png'] })))
      if (u.endsWith('/user/tokens') && init?.method === 'POST') {
        tokenCreated = true
        return Promise.resolve(jsonRes(env({ id: 9, name: 'blog', scope: 'upload', created_at: '2026-07-17T00:00:00Z', last_used_at: null, token: 'PLAIN-ONCE-TOKEN' })))
      }
      if (u.endsWith('/user/tokens'))
        return Promise.resolve(jsonRes(env(tokenCreated ? [{ id: 9, name: 'blog', scope: 'upload', created_at: '2026-07-17T00:00:00Z', last_used_at: null }] : [])))
      if (u.endsWith('/user/password') && init?.method === 'PATCH')
        return Promise.resolve(jsonRes({ status: false, message: '账号或密码错误', data: { code: 'invalid_credentials' } }, 401))
      if (u.endsWith('/user/profile') && init?.method === 'PATCH') return Promise.resolve(jsonRes(env(null)))
      if (u.endsWith('/user/preferences') && init?.method === 'PATCH') return Promise.resolve(jsonRes(env(null)))
      if (u.endsWith('/user/policies')) return Promise.resolve(jsonRes(env(policies)))
      if (u.endsWith('/albums')) return Promise.resolve(jsonRes(env({ items: [ALBUM_A, ALBUM_B] })))
      if (u.endsWith('/user') && init?.method === 'DELETE')
        return Promise.resolve(
          jsonRes({ status: false, message: '账号或密码错误', data: { code: 'invalid_credentials' } }, 401),
        )
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
}

function renderAt(path: string) {
  const qc = createQueryClient()
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/settings/:tab?" element={<SettingsPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  tokenCreated = false
  // 默认 locale 锁 zh,保证中文断言稳定
  useGlobal.setState({ toasts: [], lang: 'zh' })
  mockBackend()
})
afterEach(() => vi.unstubAllGlobals())

it('默认 profile；非法 tab 回落；不渲染清单', async () => {
  renderAt('/settings/bogus')
  expect(await screen.findByDisplayValue('凌')).toBeInTheDocument()
  expect(screen.getByDisplayValue('i@img.li')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '上传头像' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '永久注销账号' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '上传偏好' })).toBeInTheDocument()
})

it('广场开启且未开公开主页：资料页强化说明', async () => {
  mockBackend(SESSION_USER, { plazaEnabled: true })
  renderAt('/settings/profile')
  expect(await screen.findByDisplayValue('凌')).toBeInTheDocument()
  expect(screen.getByText(/本站已启用广场/)).toBeInTheDocument()
  expect(screen.getByTestId('public-profile-plaza-off')).toHaveTextContent(/不会出现在广场/)
})

it('改密码：旧密码错显示行内错误且无全局 toast', async () => {
  const user = userEvent.setup()
  renderAt('/settings')
  await screen.findByDisplayValue('凌')
  await user.type(screen.getByLabelText('当前密码'), 'wrong-old-1')
  await user.type(screen.getByLabelText('新密码'), 'newpass4word')
  await user.click(screen.getByRole('button', { name: '更新密码' }))
  expect(await screen.findByText('当前密码错误')).toBeInTheDocument()
  expect(useGlobal.getState().toasts).toHaveLength(0)
})

it('Token：新建弹窗→一次性明文条→列表刷新，吊销两击', async () => {
  const user = userEvent.setup()
  renderAt('/settings/tokens')
  expect(await screen.findByText(/暂无 Token/)).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '＋ 生成新 Token' }))
  await user.type(screen.getByPlaceholderText(/如：博客/), 'blog')
  await user.click(screen.getByRole('button', { name: /仅上传/ }))
  await user.click(screen.getByRole('button', { name: '创建 Token' }))
  expect(await screen.findByText('PLAIN-ONCE-TOKEN')).toBeInTheDocument()
  expect(await screen.findByText('blog')).toBeInTheDocument()
  expect(screen.getByText('UPLOAD')).toBeInTheDocument()
  expect(screen.getByText('PicGo')).toBeInTheDocument()
  expect(screen.getByText('ShareX')).toBeInTheDocument()
  expect(screen.getByText('curl')).toBeInTheDocument()
  expect(screen.getByText(/CLI \(imgli upload\)/)).toBeInTheDocument()
  // base_url from config + plain token injected while banner open
  expect(screen.getByText(/基址来自站点 base_url（当前 https:\/\/img\.li）/)).toBeInTheDocument()
  expect(screen.getAllByText(/Bearer PLAIN-ONCE-TOKEN/).length).toBeGreaterThan(0)
})

it('用量 Tab 大数字与进度条', async () => {
  renderAt('/settings/usage')
  expect(await screen.findByText(/2\.14 GB/)).toBeInTheDocument()
  expect(screen.getByText(/\/ 10 GB/)).toBeInTheDocument()
  expect(screen.queryByText(/升级容量/)).not.toBeInTheDocument()
})

it('未验证:显示徽章与重发按钮,点击 POST resend-verification', async () => {
  const user = userEvent.setup()
  renderAt('/settings')
  await screen.findByDisplayValue('凌')
  expect(screen.getByText('未验证')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '重发验证邮件' })).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '重发验证邮件' }))
  await waitFor(() => {
    const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls
    expect(
      calls.some(
        (c) => String(c[0]).endsWith('/auth/resend-verification') && (c[1] as RequestInit | undefined)?.method === 'POST',
      ),
    ).toBe(true)
  })
})

it('已验证:显示已验证徽章且无重发按钮', async () => {
  mockBackend({ ...SESSION_USER, email_verified: true })
  renderAt('/settings')
  await screen.findByDisplayValue('凌')
  expect(screen.getByText('已验证')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '重发验证邮件' })).not.toBeInTheDocument()
})

it('偏好 Tab 渲染与保存', async () => {
  const user = userEvent.setup()
  mockBackend()
  renderAt('/settings/preferences')
  expect(await screen.findByLabelText('默认相册')).toBeInTheDocument()
  expect(screen.getByText('默认可见性')).toBeInTheDocument()
  expect(screen.getByText('上传完成后自动复制')).toBeInTheDocument()
  expect(screen.queryByLabelText('默认存储策略')).not.toBeInTheDocument()

  await user.selectOptions(screen.getByLabelText('默认相册'), String(ALBUM_A.id))
  await user.click(screen.getByRole('button', { name: '私密' }))
  await user.click(screen.getByRole('button', { name: 'Markdown' }))
  await user.click(screen.getByRole('button', { name: '保存偏好' }))

  await waitFor(() => {
    const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls
    const patchCall = calls.find(
      (c) => String(c[0]).endsWith('/user/preferences') && (c[1] as RequestInit | undefined)?.method === 'PATCH',
    )
    expect(patchCall).toBeTruthy()
    const body = JSON.parse((patchCall![1] as RequestInit).body as string)
    expect(body).toEqual({
      default_album_id: ALBUM_A.id,
      default_visibility: 'private',
      default_policy_id: null,
      auto_copy_format: 'markdown',
      watermark: { enabled: false, position: 'br', opacity: 0.5, margin: 10 },
      // useUpdatePreferences 注入当前 UI 语言兜底(codex 基建评审 F1);测试默认 locale 锁 zh
      lang: 'zh',
    })
  })
})

it('偏好水印小节:未上传时有上传按钮无移除', async () => {
  mockBackend()
  renderAt('/settings/preferences')
  expect(await screen.findByRole('button', { name: '上传水印图(PNG)' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '移除' })).not.toBeInTheDocument()
  expect(screen.getByRole('switch', { name: '启用图片水印' })).toBeInTheDocument()
  expect(screen.getByLabelText('水印位置')).toBeInTheDocument()
})

it('偏好水印:开启+选 tl 保存 PATCH 含 watermark 且保留原字段', async () => {
  const user = userEvent.setup()
  mockBackend()
  renderAt('/settings/preferences')
  await screen.findByLabelText('默认相册')
  await user.click(screen.getByRole('switch', { name: '启用图片水印' }))
  await user.selectOptions(screen.getByLabelText('水印位置'), 'tl')
  await user.click(screen.getByRole('button', { name: '保存偏好' }))

  await waitFor(() => {
    const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls
    const patchCall = calls.find(
      (c) => String(c[0]).endsWith('/user/preferences') && (c[1] as RequestInit | undefined)?.method === 'PATCH',
    )
    expect(patchCall).toBeTruthy()
    const body = JSON.parse((patchCall![1] as RequestInit).body as string)
    expect(body.watermark.enabled).toBe(true)
    expect(body.watermark.position).toBe('tl')
    expect(body).toMatchObject({
      default_album_id: null,
      default_visibility: 'public',
      default_policy_id: null,
      auto_copy_format: '',
    })
  })
})

it('策略选择器条件渲染', async () => {
  mockBackend(SESSION_USER, {
    policies: [
      { id: 1, name: '本地' },
      { id: 2, name: 'S3' },
    ],
  })
  renderAt('/settings/preferences')
  expect(await screen.findByLabelText('默认存储策略')).toBeInTheDocument()
})

it('头像块:上传按钮;有 avatar_url 时显示移除', async () => {
  renderAt('/settings')
  expect(await screen.findByRole('button', { name: '上传头像' })).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '移除头像' })).not.toBeInTheDocument()

  mockBackend({ ...SESSION_USER, avatar_url: '/avatar/1?v=9' })
  renderAt('/settings')
  expect(await screen.findByRole('button', { name: '移除头像' })).toBeInTheDocument()
  expect(screen.getByAltText('头像')).toHaveAttribute('src', '/avatar/1?v=9')
})

it('危险区:两击注销发 DELETE /user;密码错误行内提示', async () => {
  const user = userEvent.setup()
  renderAt('/settings')
  await screen.findByDisplayValue('凌')
  await user.type(screen.getByLabelText('输入当前密码以确认'), 'wrong-pass')
  await user.click(screen.getByRole('button', { name: '永久注销账号' }))
  await user.click(screen.getByRole('button', { name: '再次点击确认注销' }))
  expect(await screen.findByText('密码错误')).toBeInTheDocument()
  await waitFor(() => {
    const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls
    const delCall = calls.find(
      (c) => String(c[0]).endsWith('/user') && (c[1] as RequestInit | undefined)?.method === 'DELETE',
    )
    expect(delCall).toBeTruthy()
    const body = JSON.parse((delCall![1] as RequestInit).body as string)
    expect(body).toEqual({ password: 'wrong-pass' })
  })
  expect(useGlobal.getState().toasts).toHaveLength(0)
})
