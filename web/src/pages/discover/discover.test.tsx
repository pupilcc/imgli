import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { App } from '../../App'
import type { DiscoverRow } from '../../api/types'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })
const notFound = (message = 'not found') =>
  jsonRes({ status: false, message, data: { code: 'not_found' } }, 404)

function row(partial: Partial<DiscoverRow> & Pick<DiscoverRow, 'key' | 'author'>): DiscoverRow {
  return {
    name: partial.name ?? partial.key,
    ext: partial.ext ?? 'jpg',
    created_at: partial.created_at ?? '2026-07-01T00:00:00Z',
    views: partial.views ?? 0,
    ...partial,
  }
}

function author(username: string, overrides: Partial<DiscoverRow['author']> = {}) {
  return {
    user_id: overrides.user_id ?? 1,
    username,
    nickname: overrides.nickname ?? username,
    avatar_version: overrides.avatar_version ?? 0,
    ...overrides,
  }
}

type FetchMock = ReturnType<typeof vi.fn>

function mockFetch(handler: (url: string, fetchMock: FetchMock) => Promise<Response> | Response | undefined) {
  const fetchMock = vi.fn((url: RequestInfo | URL) => {
    const u = String(url)
    const handled = handler(u, fetchMock)
    if (handled !== undefined) return Promise.resolve(handled)
    if (u.includes('/auth/session'))
      return Promise.resolve(jsonRes({ status: false, message: '未登录', data: { code: 'unauthorized' } }, 401))
    if (u.includes('/config'))
      return Promise.resolve(
        jsonRes(
          env({
            site_name: 'img.li',
            registration_mode: 'open',
            guest_upload_enabled: false,
            guest: null,
            plaza_enabled: true,
          }),
        ),
      )
    return Promise.resolve(notFound())
  }) as FetchMock
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
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

afterEach(() => {
  vi.unstubAllGlobals()
})

const aliceAuthor = author('alice', { user_id: 9, nickname: 'Alice' })
const row1 = row({ key: 'k1.jpg', name: 'one', author: aliceAuthor })
const row2 = row({ key: 'k2.jpg', name: 'two', author: author('bob', { user_id: 2, nickname: 'Bob' }) })

it('Explore 正常：渲染卡片与作者链接', async () => {
  mockFetch((u) => {
    if (u.includes('/plaza')) return jsonRes(env({ items: [row1, row2], next_cursor: '' }))
  })
  renderAt('/explore')
  expect(await screen.findByText('广场')).toBeInTheDocument()
  expect(await screen.findByText('Alice')).toBeInTheDocument()
  expect(screen.getByText('Bob')).toBeInTheDocument()
  const aliceLink = screen.getByRole('link', { name: /Alice/i })
  expect(aliceLink).toHaveAttribute('href', '/u/alice')
  const bobLink = screen.getByRole('link', { name: /Bob/i })
  expect(bobLink).toHaveAttribute('href', '/u/bob')
})

it('Explore 关闭态：404 显示「广场未开启」', async () => {
  mockFetch((u) => {
    if (u.includes('/plaza')) return notFound('plaza disabled')
  })
  renderAt('/explore')
  expect(await screen.findByText('广场未开启')).toBeInTheDocument()
})

it('Explore 排序切换：点热门后请求 sort=hot', async () => {
  const user = userEvent.setup()
  const fetchMock = mockFetch((u) => {
    if (u.includes('/plaza')) return jsonRes(env({ items: [row1], next_cursor: '' }))
  })
  renderAt('/explore')
  await screen.findByText('Alice')
  await user.click(screen.getByRole('button', { name: '热门' }))
  await waitFor(() => {
    const urls = fetchMock.mock.calls.map((c) => String(c[0]))
    expect(urls.some((u) => u.includes('/plaza') && u.includes('sort=hot'))).toBe(true)
  })
})

it('Explore lightbox：点卡片出现大图与复制按钮', async () => {
  const user = userEvent.setup()
  mockFetch((u) => {
    if (u.includes('/plaza')) return jsonRes(env({ items: [row1, row2], next_cursor: '' }))
  })
  renderAt('/explore')
  const authorLink = await screen.findByRole('link', { name: /Alice/i })
  const card = authorLink.closest('[role="button"]')
  expect(card).toBeTruthy()
  await user.click(card!)
  expect(await screen.findByRole('dialog')).toBeInTheDocument()
  expect(document.querySelector('img[src="/i/k1.jpg"]')).toBeTruthy()
  expect(screen.getByRole('button', { name: '复制外链' })).toBeInTheDocument()
})

it('UserPublic 正常：头部与网格', async () => {
  mockFetch((u) => {
    if (u.includes('/u/alice/images'))
      return jsonRes(
        env({
          items: [
            row({ key: 'a1.jpg', name: 'shot1', author: aliceAuthor }),
            row({ key: 'a2.jpg', name: 'shot2', author: aliceAuthor }),
          ],
          next_cursor: '',
        }),
      )
    if (u.includes('/u/alice'))
      return jsonRes(
        env({
          user: {
            username: 'alice',
            nickname: 'Alice',
            avatar_version: 1,
            joined_at: '2026-01-15T00:00:00Z',
            public_image_count: 2,
          },
        }),
      )
  })
  renderAt('/u/alice')
  expect(await screen.findByText('@alice')).toBeInTheDocument()
  expect(screen.getAllByText('Alice').length).toBeGreaterThanOrEqual(1)
  expect(screen.getByText(/2 张公开图/)).toBeInTheDocument()
  const cards = [...document.querySelectorAll('[role="button"]')].filter((el) =>
    el.querySelector('a[href="/u/alice"]'),
  )
  expect(cards.length).toBe(2)
})

it('UserPublic 404：主页不存在或未公开', async () => {
  mockFetch((u) => {
    if (u.includes('/u/ghost')) return notFound('not public')
  })
  renderAt('/u/ghost')
  expect(await screen.findByText('主页不存在或未公开')).toBeInTheDocument()
})
