import { ApiError, api, post, setOnUnauthorized, setOnForbidden } from './client'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })
const fail = (code: string, message: string) => ({ status: false, message, data: { code } })

afterEach(() => {
  vi.unstubAllGlobals()
  setOnUnauthorized(null)
  setOnForbidden(null)
})

it('成功时解开信封返回 data', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonRes(env({ used: 1, total: 2 }))))
  await expect(api('/user/quota')).resolves.toEqual({ used: 1, total: 2 })
})

it('post 自动带 JSON 头与序列化体', async () => {
  const f = vi.fn().mockResolvedValue(jsonRes(env(null)))
  vi.stubGlobal('fetch', f)
  await post('/auth/login', { account: 'a', password: 'p' })
  const [url, init] = f.mock.calls[0]
  expect(url).toBe('/api/v1/auth/login')
  expect(init.method).toBe('POST')
  expect(init.headers['Content-Type']).toBe('application/json')
  expect(JSON.parse(init.body)).toEqual({ account: 'a', password: 'p' })
})

it('业务失败抛 ApiError（码取 data.code）', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonRes(fail('quota_exceeded', '配额不足'), 413)))
  const err = (await api('/x').catch((e) => e)) as ApiError
  expect(err).toBeInstanceOf(ApiError)
  expect(err.httpStatus).toBe(413)
  expect(err.code).toBe('quota_exceeded')
  expect(err.message).toBe('配额不足')
})

it('网络失败归一为 network_error', async () => {
  vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('fetch failed')))
  const err = (await api('/x').catch((e) => e)) as ApiError
  expect(err.code).toBe('network_error')
  expect(err.httpStatus).toBe(0)
})

it('401 触发 onUnauthorized，但 invalid_credentials 不触发', async () => {
  const cb = vi.fn()
  setOnUnauthorized(cb)
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonRes(fail('unauthorized', '未登录'), 401)))
  await api('/user/profile').catch(() => {})
  expect(cb).toHaveBeenCalledOnce()

  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonRes(fail('invalid_credentials', '账号或密码错误'), 401)))
  await api('/auth/login').catch(() => {})
  expect(cb).toHaveBeenCalledOnce()
})

it('403 触发 onForbidden 钩子', async () => {
  const spy = vi.fn()
  setOnForbidden(spy)
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve({
        ok: false,
        status: 403,
        json: () => Promise.resolve({ status: false, message: '无权限', data: { code: 'forbidden' } }),
      } as unknown as Response),
    ),
  )
  await expect(api('/admin/stats')).rejects.toMatchObject({ httpStatus: 403 })
  expect(spy).toHaveBeenCalledTimes(1)
})

it('/auth/session 的 401 不触发 onUnauthorized(未登录是正常答案,游客模式依赖)', async () => {
  const cb = vi.fn()
  setOnUnauthorized(cb)
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonRes(fail('unauthorized', '未登录'), 401)))
  await expect(api('/auth/session')).rejects.toMatchObject({ httpStatus: 401 })
  expect(cb).not.toHaveBeenCalled()
})
