import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import type { ImageItem } from '../../api/types'
import { useGlobal } from '../../store'
import { ImagesPage } from './ImagesPage'

const LINKS = (k: string) => ({
  url: `http://x/i/${k}.png`, markdown: 'm', html: 'h', bbcode: 'b', thumbnail_url: `http://x/t/${k}.jpg`,
})
const item = (k: string, over: Partial<ImageItem> = {}): ImageItem => ({
  key: k, name: `${k}.png`, ext: 'png', size: 1024, width: 100, height: 80,
  visibility: 'public', album_id: null, created_at: '2026-07-16T00:00:00Z', expires_at: null, links: LINKS(k), ...over,
})

let pages: Record<string, unknown>
function jsonRes(body: unknown): Response {
  return { ok: true, status: 200, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

function mockBackend(items: ImageItem[], next = '') {
  pages = { items, next_cursor: next }
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL) => {
      const u = String(url)
      if (u.includes('/albums')) return Promise.resolve(jsonRes(env({ items: [{ id: 7, name: '工作', visibility: 'private', image_count: 1, cover_key: '', created_at: '' }] })))
      if (u.includes('/images')) return Promise.resolve(jsonRes(env(pages)))
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
}

class FakeIO {
  static instances: FakeIO[] = []
  cb: IntersectionObserverCallback
  constructor(cb: IntersectionObserverCallback) {
    this.cb = cb
    FakeIO.instances.push(this)
  }
  observe() {}
  disconnect() {}
  unobserve() {}
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <ImagesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  FakeIO.instances = []
  vi.stubGlobal('IntersectionObserver', FakeIO as unknown as typeof IntersectionObserver)
  useGlobal.setState({ toasts: [], view: 'grid' })
  localStorage.clear()
})
afterEach(() => vi.unstubAllGlobals())

it('渲染卡片与页头统计，空态与无结果态区分', async () => {
  mockBackend([item('a'), item('b', { visibility: 'private' })])
  renderPage()
  expect(await screen.findByText('a.png')).toBeInTheDocument()
  expect(screen.getByText('已加载 2 张')).toBeInTheDocument()
  expect(screen.getByText('私密', { selector: 'span' })).toBeInTheDocument() // 私密角标（区别于下拉 option）
  expect(screen.getByRole('link', { name: '回收站' })).toHaveAttribute('href', '/trash')
})

it('库空显示 EMPTY 引导，带筛选无结果显示清除筛选', async () => {
  mockBackend([])
  renderPage()
  expect(await screen.findByText('还没有图片')).toBeInTheDocument()
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'PNG' }))
  expect(await screen.findByText(/没有匹配的图片/)).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '清除全部筛选' }))
  expect(await screen.findByText('还没有图片')).toBeInTheDocument()
})

it('视图切换持久化 imgli-view', async () => {
  mockBackend([item('a')])
  renderPage()
  await screen.findByText('a.png')
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: /列表/ }))
  expect(localStorage.getItem('imgli-view')).toBe('list')
  expect(screen.getByText('文件名')).toBeInTheDocument() // 列表表头
})

it('sentinel 相交触发翻页', async () => {
  mockBackend([item('a')], 'CUR2')
  renderPage()
  await screen.findByText('a.png')
  const f = vi.mocked(fetch)
  const before = f.mock.calls.length
  pages = { items: [item('b')], next_cursor: '' }
  FakeIO.instances.at(-1)!.cb([{ isIntersecting: true } as IntersectionObserverEntry], null as never)
  await screen.findByText('b.png')
  expect(String(f.mock.calls[before][0])).toContain('cursor=CUR2')
})

it('hover 删除是两击确认，PATCH 可见性走接口', async () => {
  mockBackend([item('a')])
  renderPage()
  await screen.findByText('a.png')
  const f = vi.mocked(fetch)
  fireEvent.click(screen.getByTitle('切换可见性'))
  await waitFor(() => {
    const patchCall = f.mock.calls.find((c) => (c[1] as RequestInit)?.method === 'PATCH')
    expect(patchCall).toBeTruthy()
    expect(JSON.parse((patchCall![1] as RequestInit).body as string)).toEqual({ visibility: 'private' })
  })
  fireEvent.click(screen.getByTitle('删除'))
  expect(f.mock.calls.find((c) => (c[1] as RequestInit)?.method === 'DELETE')).toBeFalsy()
  fireEvent.click(screen.getByTitle('确认删除'))
  await waitFor(() => {
    expect(f.mock.calls.find((c) => (c[1] as RequestInit)?.method === 'DELETE')).toBeTruthy()
  })
})
