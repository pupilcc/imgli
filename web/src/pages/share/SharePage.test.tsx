import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { createQueryClient } from '../../queryClient'
import { useGlobal } from '../../store'
import { SharePage } from './SharePage'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

beforeEach(() => {
  useGlobal.setState({ toasts: [], lang: 'zh' })
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL) => {
      const u = String(url)
      if (u.includes('/config')) {
        return Promise.resolve(
          jsonRes(env({ site_name: 'img.li', registration_mode: 'open', guest_upload_enabled: false, plaza_enabled: false, guest: null })),
        )
      }
      if (u.includes('/auth/session')) {
        return Promise.resolve(jsonRes({ status: false, message: 'unauth', data: { code: 'unauthorized' } }, 401))
      }
      if (u.includes('/api/v1/s/goodkey')) {
        return Promise.resolve(
          jsonRes(
            env({
              key: 'goodkey',
              name: 'shot.png',
              ext: 'png',
              size: 1024,
              width: 100,
              height: 80,
              visibility: 'public',
              album_id: null,
              created_at: '2026-07-29T00:00:00Z',
              expires_at: null,
              share_url: 'https://img.li/s/goodkey',
              links: {
                url: 'https://img.li/i/goodkey.png',
                markdown: '![shot.png](https://img.li/i/goodkey.png)',
                html: '',
                bbcode: '',
                thumbnail_url: 'https://img.li/t/goodkey.jpg',
                share_url: 'https://img.li/s/goodkey',
              },
            }),
          ),
        )
      }
      if (u.includes('/api/v1/s/missing')) {
        return Promise.resolve(jsonRes({ status: false, message: '资源不存在', data: { code: 'not_found' } }, 404))
      }
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
})
afterEach(() => vi.unstubAllGlobals())

function renderShare(path: string) {
  const qc = createQueryClient()
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/s/:key" element={<SharePage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

it('renders preview and copy actions for public share', async () => {
  renderShare('/s/goodkey')
  expect(await screen.findByText('shot.png')).toBeInTheDocument()
  expect(screen.getByRole('img', { name: 'shot.png' })).toHaveAttribute('src', 'https://img.li/i/goodkey.png')
  expect(screen.getByText(/100×80/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /复制链接|Copy URL/i })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: /Markdown/i })).toBeInTheDocument()
})

it('shows not found for 404', async () => {
  renderShare('/s/missing')
  await waitFor(() => {
    expect(screen.getByText(/不存在|not found/i)).toBeInTheDocument()
  })
})
