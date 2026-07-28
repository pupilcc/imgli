import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { AlbumsPage } from './AlbumsPage'

function jsonRes(body: unknown): Response {
  return { ok: true, status: 200, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })
const ALBUMS = [
  { id: 1, name: '工作', visibility: 'private', image_count: 4, cover_key: 'ck1', created_at: '2026-07-16T00:00:00Z' },
  { id: 2, name: '空册', visibility: 'public', image_count: 0, cover_key: '', created_at: '2026-07-16T00:00:00Z' },
]

function mockBackend() {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      const u = String(url)
      if (u.endsWith('/albums') && (!init || !init.method)) return Promise.resolve(jsonRes(env({ items: ALBUMS })))
      if (init?.method === 'POST') return Promise.resolve(jsonRes(env({ id: 3, name: '新册', visibility: 'public' })))
      if (init?.method === 'DELETE') return Promise.resolve(jsonRes(env({ id: 1, deleted: true, with_images: false })))
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AlbumsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => mockBackend())
afterEach(() => vi.unstubAllGlobals())

it('渲染相册卡（隐私徽章/计数/封面缩略图）与新建幽灵卡', async () => {
  renderPage()
  expect(await screen.findByText('工作')).toBeInTheDocument()
  expect(screen.getByText('PRIVATE')).toBeInTheDocument()
  expect(screen.getByText(/4 张/)).toBeInTheDocument()
  expect(document.querySelector('img[src*="/t/ck1.jpg"]')).toBeTruthy()
  expect(screen.getAllByText(/新建相册/).length).toBeGreaterThan(0)
})

it('新建弹窗：名称+可见性提交 POST', async () => {
  const user = userEvent.setup()
  renderPage()
  await screen.findByText('工作')
  await user.click(screen.getByRole('button', { name: '＋ 新建相册' }))
  await user.type(screen.getByPlaceholderText(/例如：旅行/), '新册')
  await user.click(screen.getByRole('button', { name: /私密 — 仅自己可见/ }))
  await user.click(screen.getByRole('button', { name: '创建相册' }))
  await waitFor(() => {
    const f = vi.mocked(fetch)
    const call = f.mock.calls.find((c) => (c[1] as RequestInit)?.method === 'POST')
    expect(JSON.parse((call![1] as RequestInit).body as string)).toEqual({ name: '新册', visibility: 'private' })
  })
})

it('删除三选项：仅删相册发 with_images=false', async () => {
  const user = userEvent.setup()
  renderPage()
  await screen.findByText('工作')
  await user.click(screen.getAllByTitle('删除相册')[0])
  expect(await screen.findByText(/删除相册「工作」/)).toBeInTheDocument()
  expect(screen.getByText(/该相册包含 4 张图片/)).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: /仅删除相册/ }))
  await waitFor(() => {
    const f = vi.mocked(fetch)
    const call = f.mock.calls.find((c) => (c[1] as RequestInit)?.method === 'DELETE')
    expect(String(call![0])).toContain('/albums/1?with_images=false')
  })
})
