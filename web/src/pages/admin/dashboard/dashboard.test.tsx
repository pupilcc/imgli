import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { DashboardPage } from './DashboardPage'
import { densify } from './TrendChart'

vi.mock('react-chartjs-2', () => ({ Bar: () => <div data-testid="trend-chart" /> }))

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

const TRAFFIC_7D = [
  { date: '2026-07-11', views: 1 },
  { date: '2026-07-12', views: 2 },
  { date: '2026-07-13', views: 3 },
  { date: '2026-07-14', views: 4 },
  { date: '2026-07-15', views: 5 },
  { date: '2026-07-16', views: 6 },
  { date: '2026-07-17', views: 7 },
]

function mockBackend(statsExtra: Record<string, unknown> = {}) {
  vi.stubGlobal(
    'fetch',
    vi.fn((url: RequestInfo | URL) => {
      const u = String(url)
      if (u.endsWith('/admin/stats'))
        return Promise.resolve(
          jsonRes(
            env({
              users: 5,
              images: 42,
              storage: 3 * 1024 ** 3,
              today_uploads: 6,
              pending_images: 2,
              rejected_images: 1,
              tasks_pending: 3,
              tasks_running: 0,
              daily: [{ date: '2026-07-17', count: 6 }],
              traffic_7d: TRAFFIC_7D,
              traffic_30d: TRAFFIC_7D,
              top_referers: [
                { host: 'a.example', count: 12, suspect: true },
                { host: 'b.example', count: 3 },
              ],
              top_referers_30d: [
                { host: 'a.example', count: 12, suspect: true },
                { host: 'b.example', count: 3 },
              ],
              signups_30d: [{ date: '2026-07-17', count: 2 }],
              signup_channels_30d: [{ channel: 'direct', count: 2 }],
              bandwidth_used_month: 1024,
              bandwidth_top_users: [],
              origin_metering_only: true,
              ...statsExtra,
            }),
          ),
        )
      if (u.includes('/admin/logs'))
        return Promise.resolve(
          jsonRes(
            env({
              items: [
                { id: 2, actor_id: 1, actor_type: 'admin', action: 'review_reject', detail: '{}', ip: '::1', created_at: '2026-07-17T10:30:00Z' },
                { id: 1, actor_id: 1, actor_type: 'admin', action: 'moderation_flag', detail: '{}', ip: '::1', created_at: '2026-07-17T09:00:00Z' },
                { id: 0, actor_id: 1, actor_type: 'admin', action: 'future_action', detail: '{}', ip: '::1', created_at: '2026-07-17T08:00:00Z' },
              ],
              total: 3,
              page: 1,
              limit: 8,
            }),
          ),
        )
      return Promise.resolve(jsonRes(env(null)))
    }),
  )
}

afterEach(() => vi.unstubAllGlobals())

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

it('统计卡:数值与容量格式化 + 队列指标', async () => {
  mockBackend()
  renderPage()
  expect(await screen.findByText('42')).toBeInTheDocument()
  expect(screen.getByText('5')).toBeInTheDocument()
  expect(screen.getByText('6')).toBeInTheDocument()
  expect(screen.getByText('3 GB')).toBeInTheDocument()
  expect(screen.getByText('用户数')).toBeInTheDocument()
  expect(screen.getByText('待审图片')).toBeInTheDocument()
  expect(screen.getByText('任务排队')).toBeInTheDocument()
  expect(screen.getByText('已拒绝')).toBeInTheDocument()
  expect(screen.getByText('任务执行中')).toBeInTheDocument()
})

it('趋势图挂载(桩)+ 最近事件文案', async () => {
  mockBackend()
  renderPage()
  expect((await screen.findAllByTestId('trend-chart')).length).toBeGreaterThanOrEqual(1)
  expect(await screen.findByText('审核拒绝')).toBeInTheDocument()
  expect(screen.getByText('机审标记')).toBeInTheDocument()
  expect(screen.getByText('future_action')).toBeInTheDocument() // 未知 action 显示原码
})

it('运营流量 + 来源表 + 源站脚注', async () => {
  mockBackend()
  renderPage()
  expect(await screen.findByText('30 日流量')).toBeInTheDocument()
  expect(screen.getByText('来源 Top')).toBeInTheDocument()
  expect(screen.getByText('a.example')).toBeInTheDocument()
  expect(screen.getByText('b.example')).toBeInTheDocument()
  expect(screen.getAllByText(/仅统计源站可见访问/).length).toBeGreaterThanOrEqual(1)
  expect(screen.getAllByTestId('trend-chart').length).toBeGreaterThanOrEqual(3)
})

it('top_referers 空:显示暂无外链访问', async () => {
  mockBackend({ top_referers: [], top_referers_30d: [] })
  renderPage()
  expect(await screen.findByText('暂无外链访问')).toBeInTheDocument()
})

it('densify:铺满 30 天,缺日补 0', () => {
  const out = densify([])
  expect(out).toHaveLength(30)
  expect(out.every((d) => d.count === 0)).toBe(true)
  const today = new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  const key = `${today.getFullYear()}-${p(today.getMonth() + 1)}-${p(today.getDate())}`
  expect(out[29].date).toBe(key)
  const out2 = densify([{ date: key, count: 9 }])
  expect(out2[29].count).toBe(9)
})

it('stats 失败:渲染错误态而非卡骨架', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() =>
      Promise.resolve(jsonRes({ status: false, message: '服务器内部错误', data: { code: 'internal_error' } }, 500)),
    ),
  )
  renderPage()
  expect(await screen.findByText('加载失败')).toBeInTheDocument()
})

it('moderation_flag 显示中文标签', async () => {
  mockBackend()
  renderPage()
  expect(await screen.findByText('机审标记')).toBeInTheDocument()
})
