import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import {
  useAdminLogs, useAdminStats, useReviewCount, useAdminUsers, useUpdateAdminUser, useSetImageWhitelist, useResetAdminPassword, useDeleteAdminImage,
  useAdminReview, useReviewDecide, useReviewBatch,
  useCreateGroup, useUpdateGroup, useDeleteGroup,
  useTestPolicy,
  useAdminSettings, useUpdateSettings,
} from './adminHooks'
import { useGlobal } from '../store'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

function wrap() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const Wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  )
  return { qc, Wrapper }
}

afterEach(() => vi.unstubAllGlobals())

it('useAdminStats 解包 /admin/stats', async () => {
  const fetchMock = vi.fn(() =>
    Promise.resolve(jsonRes(env({ users: 5, images: 42, storage: 1024, today_uploads: 3, daily: null }))),
  )
  vi.stubGlobal('fetch', fetchMock)
  const { Wrapper } = wrap()
  const { result } = renderHook(() => useAdminStats(), { wrapper: Wrapper })
  await waitFor(() => expect(result.current.data).toBeDefined())
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/stats', expect.anything())
  expect(result.current.data!.users).toBe(5)
  expect(result.current.data!.daily).toBeNull()
})

it('useAdminLogs 组装筛选参数', async () => {
  const fetchMock = vi.fn(() => Promise.resolve(jsonRes(env({ items: [], total: 0, page: 2, limit: 8 }))))
  vi.stubGlobal('fetch', fetchMock)
  const { Wrapper } = wrap()
  const { result } = renderHook(() => useAdminLogs({ action: 'user_update', page: 2, limit: 8 }), { wrapper: Wrapper })
  await waitFor(() => expect(result.current.data).toBeDefined())
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/logs?action=user_update&page=2&limit=8', expect.anything())
})

it('useReviewCount 用 limit=1 取 total', async () => {
  const fetchMock = vi.fn(() => Promise.resolve(jsonRes(env({ items: [{ key: 'k' }], total: 7, page: 1, limit: 1 }))))
  vi.stubGlobal('fetch', fetchMock)
  const { Wrapper } = wrap()
  const { result } = renderHook(() => useReviewCount(), { wrapper: Wrapper })
  await waitFor(() => expect(result.current.data).toBe(7))
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/review?limit=1', expect.anything())
})

it('useAdminUsers 组装筛选参数', async () => {
  const fetchMock = vi.fn(() => Promise.resolve(jsonRes(env({ items: [], total: 0, page: 2, limit: 50 }))))
  vi.stubGlobal('fetch', fetchMock)
  const { Wrapper } = wrap()
  const { result } = renderHook(() => useAdminUsers({ q: 'ling', group: 2, status: 'banned', page: 2 }), { wrapper: Wrapper })
  await waitFor(() => expect(result.current.data).toBeDefined())
  expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/users?q=ling&group=2&status=banned&page=2', expect.anything())
})

it('useUpdateAdminUser PATCH 后 invalidate users 列表并 toast 本地化错误', async () => {
  const calls: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      calls.push(`${init?.method ?? 'GET'} ${url}`)
      if (init?.method === 'PATCH')
        return Promise.resolve(jsonRes({ status: false, message: '不能封禁自己', data: { code: 'invalid_request' } }, 400))
      return Promise.resolve(jsonRes(env({ items: [], total: 0, page: 1, limit: 50 })))
    }),
  )
  const { Wrapper } = wrap()
  const { result } = renderHook(() => useUpdateAdminUser(), { wrapper: Wrapper })
  result.current.mutate({ id: 1, body: { status: 'banned' } })
  await waitFor(() => expect(result.current.isError).toBe(true))
  // zh locale:errorText 恒用后端细分 message(不被通用 errors.invalid_request 覆盖,不回归)
  expect(useGlobal.getState().toasts.some((t) => t.message === '不能封禁自己')).toBe(true)
})

it('useUpdateAdminUser 成功后同时失效 users 与 groups', async () => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(jsonRes(env({ id: 2, group_id: 2 })))))
  const { qc, Wrapper } = wrap()
  const spy = vi.spyOn(qc, 'invalidateQueries')
  const { result } = renderHook(() => useUpdateAdminUser(), { wrapper: Wrapper })
  result.current.mutate({ id: 2, body: { group_id: 2 } })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  const keys = spy.mock.calls.map((c) => JSON.stringify((c[0] as { queryKey: unknown }).queryKey))
  expect(keys).toContain(JSON.stringify(['admin', 'users']))
  expect(keys).toContain(JSON.stringify(['admin', 'groups']))
})

it('useSetImageWhitelist 成功后 invalidate images', async () => {
  const fetchMock = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
    if (init?.method === 'PATCH') return Promise.resolve(jsonRes(env({ key: 'k1', is_whitelisted: true })))
    return Promise.resolve(jsonRes(env({ items: [], total: 0, page: 1, limit: 50 })))
  })
  vi.stubGlobal('fetch', fetchMock)
  const { Wrapper } = wrap()
  const { result } = renderHook(() => useSetImageWhitelist(), { wrapper: Wrapper })
  result.current.mutate({ key: 'k1', on: true })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  expect(fetchMock).toHaveBeenCalledWith(
    '/api/v1/admin/images/k1',
    expect.objectContaining({ method: 'PATCH', body: JSON.stringify({ is_whitelisted: true }) }),
  )
})

it('useSetImageWhitelist 成功后同时失效 images 与 review-count', async () => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(jsonRes(env({ key: 'k1', is_whitelisted: true })))))
  const { qc, Wrapper } = wrap()
  const spy = vi.spyOn(qc, 'invalidateQueries')
  const { result } = renderHook(() => useSetImageWhitelist(), { wrapper: Wrapper })
  result.current.mutate({ key: 'k1', on: true })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  const keys = spy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey))
  expect(keys).toContain(JSON.stringify(['admin', 'images']))
  expect(keys).toContain(JSON.stringify(['admin', 'review-count']))
})

it('useResetAdminPassword 返回一次性密码', async () => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(jsonRes(env({ password: 'NEW-PASS-123' })))))
  const { Wrapper } = wrap()
  const { result } = renderHook(() => useResetAdminPassword(), { wrapper: Wrapper })
  result.current.mutate(5)
  await waitFor(() => expect(result.current.data?.password).toBe('NEW-PASS-123'))
})

it('useDeleteAdminImage 成功后同时失效 images 与 review-count', async () => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(jsonRes(env({ key: 'k1', deleted: true })))))
  const { qc, Wrapper } = wrap()
  const spy = vi.spyOn(qc, 'invalidateQueries')
  const { result } = renderHook(() => useDeleteAdminImage(), { wrapper: Wrapper })
  result.current.mutate('k1')
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  const keys = spy.mock.calls.map((c) => JSON.stringify(c[0]?.queryKey))
  expect(keys).toContain(JSON.stringify(['admin', 'images']))
  expect(keys).toContain(JSON.stringify(['admin', 'review-count']))
})

it('useAdminReview:page>1 才带 page 参数', async () => {
  const fetchMock = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) =>
    Promise.resolve(jsonRes(env({ items: [], total: 0, page: 2, limit: 50 }))),
  )
  vi.stubGlobal('fetch', fetchMock)
  const { Wrapper } = wrap()
  renderHook(() => useAdminReview(2), { wrapper: Wrapper })
  await waitFor(() => expect(fetchMock).toHaveBeenCalled())
  expect(String(fetchMock.mock.calls[0][0])).toContain('/admin/review?page=2')
})

it('useReviewDecide:POST /admin/review/{key} 带 action,成功失效 review/review-count/images', async () => {
  const fetchMock = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) =>
    Promise.resolve(jsonRes(env({ key: 'k1', status: 'normal' }))),
  )
  vi.stubGlobal('fetch', fetchMock)
  const { qc, Wrapper } = wrap()
  const spy = vi.spyOn(qc, 'invalidateQueries')
  const { result } = renderHook(() => useReviewDecide(), { wrapper: Wrapper })
  result.current.mutate({ key: 'k1', action: 'approve' })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  const [url, init] = fetchMock.mock.calls[0]
  expect(String(url)).toContain('/admin/review/k1')
  expect(JSON.parse(String((init as RequestInit).body))).toEqual({ action: 'approve' })
  const keys = spy.mock.calls.map((c) => JSON.stringify((c[0] as { queryKey: unknown }).queryKey))
  expect(keys).toContain(JSON.stringify(['admin', 'review']))
  expect(keys).toContain(JSON.stringify(['admin', 'review-count']))
  expect(keys).toContain(JSON.stringify(['admin', 'images']))
})

it('useReviewBatch:POST /admin/review/batch 带 keys+action', async () => {
  const fetchMock = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) =>
    Promise.resolve(jsonRes(env({ results: [{ key: 'a', ok: true }] }))),
  )
  vi.stubGlobal('fetch', fetchMock)
  const { Wrapper } = wrap()
  const { result } = renderHook(() => useReviewBatch(), { wrapper: Wrapper })
  result.current.mutate({ keys: ['a', 'b'], action: 'approve' })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  const [url, init] = fetchMock.mock.calls[0]
  expect(String(url)).toContain('/admin/review/batch')
  expect(JSON.parse(String((init as RequestInit).body))).toEqual({ keys: ['a', 'b'], action: 'approve' })
})

it('useReviewBatch 成功后失效 review/review-count/images', async () => {
  vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(jsonRes(env({ results: [] })))))
  const { qc, Wrapper } = wrap()
  const spy = vi.spyOn(qc, 'invalidateQueries')
  const { result } = renderHook(() => useReviewBatch(), { wrapper: Wrapper })
  result.current.mutate({ keys: ['a'], action: 'approve' })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  const keys = spy.mock.calls.map((c) => JSON.stringify((c[0] as { queryKey: unknown }).queryKey))
  expect(keys).toContain(JSON.stringify(['admin', 'review']))
  expect(keys).toContain(JSON.stringify(['admin', 'review-count']))
  expect(keys).toContain(JSON.stringify(['admin', 'images']))
})

it('useCreateGroup / useUpdateGroup / useDeleteGroup:方法与路径', async () => {
  const fetchMock = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) => Promise.resolve(jsonRes(env({ id: 3 }))))
  vi.stubGlobal('fetch', fetchMock)
  const { Wrapper } = wrap()
  const c = renderHook(() => useCreateGroup(), { wrapper: Wrapper })
  c.result.current.mutate({ name: 'g' })
  await waitFor(() => expect(c.result.current.isSuccess).toBe(true))
  expect((fetchMock.mock.calls[0][1] as RequestInit).method).toBe('POST')
  const u = renderHook(() => useUpdateGroup(), { wrapper: Wrapper })
  u.result.current.mutate({ id: 3, body: { name: 'g2' } })
  await waitFor(() => expect(u.result.current.isSuccess).toBe(true))
  expect(String(fetchMock.mock.calls[1][0])).toContain('/admin/groups/3')
  expect((fetchMock.mock.calls[1][1] as RequestInit).method).toBe('PATCH')
  const d = renderHook(() => useDeleteGroup(), { wrapper: Wrapper })
  d.result.current.mutate(3)
  await waitFor(() => expect(d.result.current.isSuccess).toBe(true))
  expect((fetchMock.mock.calls[2][1] as RequestInit).method).toBe('DELETE')
})

it('useTestPolicy:POST /admin/policies/{id}/test', async () => {
  const fetchMock = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) =>
    Promise.resolve(jsonRes(env({ ok: true, latency_ms: 7 }))),
  )
  vi.stubGlobal('fetch', fetchMock)
  const { Wrapper } = wrap()
  const { result } = renderHook(() => useTestPolicy(), { wrapper: Wrapper })
  result.current.mutate(9)
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  expect(String(fetchMock.mock.calls[0][0])).toContain('/admin/policies/9/test')
  expect(result.current.data).toEqual({ ok: true, latency_ms: 7 })
})

const SETTINGS = {
  site_name: '白栗',
  registration_mode: 'open',
  moderation: {
    enabled: false,
    provider: 'webhook',
    endpoint: '',
    api_key: '****cdef',
    access_key_id: '',
    access_key_secret: '',
    region: '',
    threshold: 0.8,
    action: 'pending',
  },
}

it('useAdminSettings:GET /admin/settings', async () => {
  const fetchMock = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) => Promise.resolve(jsonRes(env(SETTINGS))))
  vi.stubGlobal('fetch', fetchMock)
  const { Wrapper } = wrap()
  const { result } = renderHook(() => useAdminSettings(), { wrapper: Wrapper })
  await waitFor(() => expect(result.current.data).toBeTruthy())
  expect(String(fetchMock.mock.calls[0][0])).toContain('/admin/settings')
  expect(result.current.data).toEqual(SETTINGS)
})

it('useUpdateSettings:PUT /admin/settings,成功失效 settings', async () => {
  const fetchMock = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) => Promise.resolve(jsonRes(env(SETTINGS))))
  vi.stubGlobal('fetch', fetchMock)
  const { qc, Wrapper } = wrap()
  const spy = vi.spyOn(qc, 'invalidateQueries')
  const { result } = renderHook(() => useUpdateSettings(), { wrapper: Wrapper })
  result.current.mutate({ site_name: '新名' })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  const [url, init] = fetchMock.mock.calls[0]
  expect(String(url)).toContain('/admin/settings')
  expect((init as RequestInit).method).toBe('PUT')
  expect(JSON.parse(String((init as RequestInit).body))).toEqual({ site_name: '新名' })
  const keys = spy.mock.calls.map((c) => JSON.stringify((c[0] as { queryKey: unknown }).queryKey))
  expect(keys).toContain(JSON.stringify(['admin', 'settings']))
  expect(keys).toContain(JSON.stringify(['config']))
})
