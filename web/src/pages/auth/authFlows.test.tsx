import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { ForgotPasswordPage } from './ForgotPasswordPage'
import { ResetPasswordPage } from './ResetPasswordPage'
import { VerifyEmailPage } from './VerifyEmailPage'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

function renderAt(path: string, el: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path={path.split('?')[0]} element={el} />
          <Route path="/login" element={<div>LOGIN</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

function findCall(f: ReturnType<typeof vi.fn>, suffix: string) {
  return f.mock.calls.find((c) => String(c[0]).endsWith(suffix))
}

it('忘记密码:提交后恒显成功文案', async () => {
  const f = vi.fn().mockResolvedValue(jsonRes(env({})))
  vi.stubGlobal('fetch', f)
  renderAt('/forgot-password', <ForgotPasswordPage />)
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('邮箱'), 'a@img.li')
  await user.click(screen.getByRole('button', { name: '发送重置邮件' }))
  expect(await screen.findByText(/若该邮箱已注册/)).toBeInTheDocument()
  const call = findCall(f, '/auth/forgot-password')
  expect(call).toBeDefined()
  expect(String(call![0])).toBe('/api/v1/auth/forgot-password')
})

it('重置密码:读 token 提交,成功引导去登录', async () => {
  const f = vi.fn().mockResolvedValue(jsonRes(env({})))
  vi.stubGlobal('fetch', f)
  renderAt('/reset-password?token=T123', <ResetPasswordPage />)
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('新密码'), 'newpass-111')
  await user.type(screen.getByLabelText('确认新密码'), 'newpass-111')
  await user.click(screen.getByRole('button', { name: '重置密码' }))
  expect(await screen.findByText(/密码已重置/)).toBeInTheDocument()
  const call = findCall(f, '/auth/reset-password')
  expect(call).toBeDefined()
  const body = JSON.parse(String((call![1] as RequestInit).body))
  expect(body).toEqual({ token: 'T123', password: 'newpass-111' })
})

it('重置密码:两次输入不一致行内错', async () => {
  vi.stubGlobal('fetch', vi.fn())
  renderAt('/reset-password?token=T123', <ResetPasswordPage />)
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('新密码'), 'newpass-111')
  await user.type(screen.getByLabelText('确认新密码'), 'different-1')
  await user.click(screen.getByRole('button', { name: '重置密码' }))
  expect(screen.getByText('两次输入的密码不一致')).toBeInTheDocument()
})

it('验证邮箱:挂载即调 API,成功显成功态', async () => {
  const f = vi.fn().mockResolvedValue(jsonRes(env({})))
  vi.stubGlobal('fetch', f)
  renderAt('/verify-email?token=V1', <VerifyEmailPage />)
  expect(await screen.findByText('邮箱验证成功')).toBeInTheDocument()
  const call = findCall(f, '/auth/verify-email')
  expect(call).toBeDefined()
  expect(JSON.parse(String((call![1] as RequestInit).body))).toEqual({ token: 'V1' })
})

it('验证邮箱:失效 token 显失败态', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonRes({ status: false, message: '链接无效或已过期', data: { code: 'token_invalid' } }, 400)))
  renderAt('/verify-email?token=V1', <VerifyEmailPage />)
  expect(await screen.findByText(/链接无效或已过期/)).toBeInTheDocument()
})
