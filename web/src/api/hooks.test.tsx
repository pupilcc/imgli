import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { defaultFilter, useImages, useCreateToken, useDeleteAlbum, useTrash, useConfig, useQuota } from './hooks'

function jsonRes(body: unknown): Response {
  return { ok: true, status: 200, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

afterEach(() => vi.unstubAllGlobals())

it('useImages 组装筛选参数并用 next_cursor 翻页', async () => {
  const f = vi
    .fn()
    .mockResolvedValueOnce(jsonRes(env({ items: [{ key: 'a' }], next_cursor: 'CUR2' })))
    .mockResolvedValueOnce(jsonRes(env({ items: [{ key: 'b' }], next_cursor: '' })))
  vi.stubGlobal('fetch', f)
  const { result } = renderHook(
    () => useImages({ ...defaultFilter, q: 'cat', format: 'PNG', album: 7, visibility: 'private', sort: 'size' }),
    { wrapper },
  )
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  const url1 = String(f.mock.calls[0][0])
  expect(url1).toContain('/api/v1/images?')
  expect(url1).toContain('q=cat')
  expect(url1).toContain('format=PNG')
  expect(url1).toContain('album=7')
  expect(url1).toContain('visibility=private')
  expect(url1).toContain('sort=size')
  expect(url1).toContain('limit=24')
  expect(url1).not.toContain('cursor=')

  await waitFor(() => expect(result.current.hasNextPage).toBe(true))

  await act(async () => {
    await result.current.fetchNextPage()
  })
  await waitFor(() => expect(result.current.data?.pages).toHaveLength(2))
  expect(String(f.mock.calls[1][0])).toContain('cursor=CUR2')
  await waitFor(() => expect(result.current.hasNextPage).toBe(false))
})

it('useImages 默认筛选省略空参数', async () => {
  const f = vi.fn().mockResolvedValue(jsonRes(env({ items: [], next_cursor: '' })))
  vi.stubGlobal('fetch', f)
  const { result } = renderHook(() => useImages(defaultFilter), { wrapper })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  const url = String(f.mock.calls[0][0])
  expect(url).not.toContain('q=')
  expect(url).not.toContain('format=')
  expect(url).not.toContain('album=')
  expect(url).not.toContain('visibility=')
  expect(url).not.toContain('sort=')
})

it('useTrash 走 /trash 且游标翻页', async () => {
  const f = vi
    .fn()
    .mockResolvedValueOnce(jsonRes(env({ items: [{ key: 't1' }], next_cursor: 'C2' })))
    .mockResolvedValueOnce(jsonRes(env({ items: [{ key: 't2' }], next_cursor: '' })))
  vi.stubGlobal('fetch', f)
  const { result } = renderHook(() => useTrash(), { wrapper })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  expect(String(f.mock.calls[0][0])).toContain('/api/v1/trash?')
  await waitFor(() => expect(result.current.hasNextPage).toBe(true))
  await act(async () => {
    await result.current.fetchNextPage()
  })
  expect(String(f.mock.calls[1][0])).toContain('cursor=C2')
})

it('useDeleteAlbum 携带 with_images 并失效 albums+images', async () => {
  const f = vi.fn().mockResolvedValue(jsonRes(env({ id: 7, deleted: true, with_images: true })))
  vi.stubGlobal('fetch', f)
  const { result } = renderHook(() => useDeleteAlbum(), { wrapper })
  await act(async () => {
    await result.current.mutateAsync({ id: 7, withImages: true })
  })
  const [url, init] = f.mock.calls[0]
  expect(String(url)).toBe('/api/v1/albums/7?with_images=true')
  expect((init as RequestInit).method).toBe('DELETE')
})

it('useCreateToken 返回一次性明文', async () => {
  const f = vi.fn().mockResolvedValue(
    jsonRes(env({ id: 1, name: 'blog', scope: 'upload', created_at: '', last_used_at: null, token: 'PLAIN-ONCE' })),
  )
  vi.stubGlobal('fetch', f)
  const { result } = renderHook(() => useCreateToken(), { wrapper })
  let out: { token?: string } | undefined
  await act(async () => {
    out = await result.current.mutateAsync({ name: 'blog', scope: 'upload' })
  })
  expect(out?.token).toBe('PLAIN-ONCE')
  expect(JSON.parse((f.mock.calls[0][1] as RequestInit).body as string)).toEqual({ name: 'blog', scope: 'upload' })
})

it('useConfig 请求公开 /config 并返回配置', async () => {
  const f = vi.fn().mockResolvedValue(jsonRes(env({
    site_name: 'img.li', registration_mode: 'open', guest_upload_enabled: true,
    guest: { max_file_size: 5 * 1024 ** 2, allowed_exts: ['png'], per_day: 3 },
  })))
  vi.stubGlobal('fetch', f)
  const { result } = renderHook(() => useConfig(), { wrapper })
  await waitFor(() => expect(result.current.isSuccess).toBe(true))
  expect(String(f.mock.calls[0][0])).toBe('/api/v1/config')
  expect(result.current.data?.guest_upload_enabled).toBe(true)
  expect(result.current.data?.guest?.per_day).toBe(3)
})

it('useQuota(false) 不发请求(游客态防 401)', async () => {
  const f = vi.fn()
  vi.stubGlobal('fetch', f)
  renderHook(() => useQuota(false), { wrapper })
  await new Promise((r) => setTimeout(r, 20))
  expect(f).not.toHaveBeenCalled()
})
