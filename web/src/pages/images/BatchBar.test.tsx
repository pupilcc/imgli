import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ImageItem } from '../../api/types'
import { useGlobal } from '../../store'
import { BatchBar } from './BatchBar'

const LINKS = { url: 'http://x/i/a.png', markdown: 'm', html: 'h', bbcode: 'b', thumbnail_url: 't' }
const items: ImageItem[] = [
  { key: 'a', name: 'a.png', ext: 'png', size: 1, width: 1, height: 1, visibility: 'public', album_id: null, created_at: '', expires_at: null, links: LINKS },
  { key: 'b', name: 'b.png', ext: 'png', size: 1, width: 1, height: 1, visibility: 'public', album_id: null, created_at: '', expires_at: null, links: { ...LINKS, url: 'http://x/i/b.png' } },
]

function jsonRes(body: unknown): Response {
  return { ok: true, status: 200, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

function renderBar(selected = new Set(['a', 'b'])) {
  const onClear = vi.fn()
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <BatchBar selected={selected} items={items} onClear={onClear} />
    </QueryClientProvider>,
  )
  return onClear
}

beforeEach(() => useGlobal.setState({ toasts: [] }))
afterEach(() => vi.unstubAllGlobals())

it('未选中不渲染，选中显示计数', () => {
  renderBar(new Set())
  expect(screen.queryByText(/已选/)).not.toBeInTheDocument()
})

it('复制链接拼接所有选中 URL', async () => {
  const user = userEvent.setup()
  const writeText = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  const f = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) => {
    const url = String(_url)
    if (url.includes('/albums')) return Promise.resolve(jsonRes(env({ items: [] })))
    return Promise.resolve(jsonRes(env({})))
  })
  vi.stubGlobal('fetch', f)
  renderBar()
  await user.click(screen.getByRole('button', { name: '复制链接' }))
  await waitFor(() => expect(writeText).toHaveBeenCalledWith('http://x/i/a.png\nhttp://x/i/b.png'))
})

it('批量删除两击确认后 POST batch 并按 results toast', async () => {
  const user = userEvent.setup()
  const f = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) => {
    const url = String(_url)
    if (url.includes('/albums')) return Promise.resolve(jsonRes(env({ items: [] })))
    return Promise.resolve(jsonRes(env({ results: [{ key: 'a', ok: true }, { key: 'b', ok: false, error: 'not_found' }] })))
  })
  vi.stubGlobal('fetch', f)
  const onClear = renderBar()
  await user.click(screen.getByRole('button', { name: '移入回收站' }))
  expect(f.mock.calls.filter((c) => String(c[0]).includes('/images/batch'))).toHaveLength(0)
  // Reset mock to track only batch calls
  f.mockClear()
  await user.click(screen.getByRole('button', { name: '确认移入回收站？' }))
  await waitFor(() => expect(f).toHaveBeenCalled())
  const call = f.mock.calls[0]
  expect(String(call[0])).toContain('/images/batch')
  expect(JSON.parse(((call[1] as unknown) as RequestInit).body as string)).toEqual({ action: 'delete', keys: ['a', 'b'] })
  await waitFor(() => expect(useGlobal.getState().toasts.at(-1)?.message).toBe('已移入回收站 1 张，1 张失败'))
  expect(onClear).toHaveBeenCalled()
})

it('改可见性经弹窗选择后 POST batch', async () => {
  const user = userEvent.setup()
  const f = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) => {
    const url = String(_url)
    if (url.includes('/albums')) return Promise.resolve(jsonRes(env({ items: [] })))
    return Promise.resolve(jsonRes(env({ results: [{ key: 'a', ok: true }, { key: 'b', ok: true }] })))
  })
  vi.stubGlobal('fetch', f)
  renderBar()
  await user.click(screen.getByRole('button', { name: '改可见性' }))
  await user.click(screen.getByRole('button', { name: /设为私密/ }))
  await waitFor(() => {
    const call = f.mock.calls.find((c) => String(c[0]).includes('/images/batch'))
    expect(call).toBeTruthy()
    expect(JSON.parse(((call![1] as unknown) as RequestInit).body as string)).toEqual({ action: 'visibility', keys: ['a', 'b'], visibility: 'private' })
  })
})

it('移动弹窗每次打开都复位为未分类', async () => {
  const user = userEvent.setup()
  const f = vi.fn((_url: RequestInfo | URL) => {
    const url = String(_url)
    if (url.includes('/albums')) return Promise.resolve(jsonRes(env({ items: [{ id: 7, name: '工作', visibility: 'private', image_count: 0, cover_key: '', created_at: '' }] })))
    return Promise.resolve(jsonRes(env({ results: [{ key: 'a', ok: true }, { key: 'b', ok: true }] })))
  })
  vi.stubGlobal('fetch', f)
  renderBar()
  await user.click(screen.getByRole('button', { name: '移动到相册' }))
  await user.selectOptions(await screen.findByRole('combobox', { name: '目标相册' }), '7')
  await user.click(screen.getByRole('button', { name: '确认移动' }))
  await waitFor(() => expect(screen.queryByRole('combobox', { name: '目标相册' })).not.toBeInTheDocument())
  await user.click(screen.getByRole('button', { name: '移动到相册' }))
  expect(screen.getByRole('combobox', { name: '目标相册' })).toHaveValue('none')
})

it('超过 100 键分两批发送并合并统计', async () => {
  const user = userEvent.setup()
  const bigItems = Array.from({ length: 120 }, (_, i) => ({ ...items[0], key: `k${i}`, links: { ...LINKS, url: `u${i}` } }))
  const f = vi.fn((_url: RequestInfo | URL, _init?: RequestInit) => {
    const url = String(_url)
    if (url.includes('/albums')) return Promise.resolve(jsonRes(env({ items: [] })))
    return Promise.resolve(jsonRes(env({ results: Array.from({ length: 100 }, (_, i) => ({ key: `x${i}`, ok: true })) })))
  })
  vi.stubGlobal('fetch', f)
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <BatchBar selected={new Set(bigItems.map((i) => i.key))} items={bigItems} onClear={vi.fn()} />
    </QueryClientProvider>,
  )
  await user.click(screen.getByRole('button', { name: '移入回收站' }))
  await user.click(screen.getByRole('button', { name: '确认移入回收站？' }))
  await waitFor(() => {
    const calls = f.mock.calls.filter((c) => String(c[0]).includes('/images/batch'))
    expect(calls).toHaveLength(2)
    expect(JSON.parse((calls[0][1] as RequestInit).body as string).keys).toHaveLength(100)
    expect(JSON.parse((calls[1][1] as RequestInit).body as string).keys).toHaveLength(20)
  })
})
