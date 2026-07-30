import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router'
import { AnnouncementBar, SiteFooter } from './SiteSlots'

function wrap(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}

/** Regression: locale-map fields must not crash React (prod white-screen outage). */
it('AnnouncementBar + SiteFooter render locale maps without throwing', () => {
  wrap(
    <>
      <AnnouncementBar
        announcement={{
          enabled: true,
          text: { zh: '中文公告', en: 'English notice' },
          link_url: 'https://example.com',
          link_label: { zh: '说明', en: 'Details' },
          dismissible: true,
          starts_at: '',
          ends_at: '',
        }}
      />
      <SiteFooter
        siteName="img.li - 图鲤"
        footer={{
          groups: [
            {
              title: { zh: '产品', en: 'Product' },
              links: [
                { label: { zh: '文档', en: 'Docs' }, url: 'https://docs.imgli.com' },
                { label: { zh: 'GitHub', en: 'GitHub' }, url: 'https://github.com/yixian-huang/imgli' },
              ],
            },
            {
              title: { zh: '社区', en: 'Community' },
              links: [{ label: { zh: 'Issues', en: 'Issues' }, url: 'https://github.com/yixian-huang/imgli/issues' }],
            },
          ],
        }}
      />
    </>,
  )
  expect(screen.getByText('中文公告')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '说明' })).toHaveAttribute('href', 'https://example.com')
  expect(screen.getByText('产品')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '文档' })).toBeInTheDocument()
  expect(screen.getByText(/©/)).toBeInTheDocument()
})

it('legacy string announcement still works', () => {
  wrap(
    <AnnouncementBar
      announcement={{
        enabled: true,
        text: 'legacy string',
        link_url: '',
        link_label: '',
        dismissible: false,
        starts_at: '',
        ends_at: '',
      }}
    />,
  )
  expect(screen.getByText('legacy string')).toBeInTheDocument()
})
