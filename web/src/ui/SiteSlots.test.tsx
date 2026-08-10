import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router'
import { afterEach, vi } from 'vitest'
import { AnnouncementBar, HtmlInject, SiteFooter } from './SiteSlots'

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

afterEach(() => {
  document.querySelectorAll('script[data-imgli-test-script]').forEach((n) => n.remove())
  document.querySelectorAll('[data-imgli-test-wrap]').forEach((n) => n.remove())
})

// jsdom 不真正执行 script；断言「可执行形态」：createElement 重建 + 属性/正文保留。
// 旧实现 cloneNode(innerHTML) 插入的 script 在浏览器里也不会执行。
it('HtmlInject mounts inline script via createElement (executable form)', async () => {
  const created: HTMLScriptElement[] = []
  const orig = document.createElement.bind(document)
  const spy = vi.spyOn(document, 'createElement').mockImplementation((tagName: string, options?: ElementCreationOptions) => {
    const el = orig(tagName, options)
    if (String(tagName).toLowerCase() === 'script') created.push(el as HTMLScriptElement)
    return el
  })
  try {
    wrap(
      <HtmlInject
        inject={{
          head: '',
          body_end: '<script data-imgli-test-script>window.__imgliInjectRan=true</script>',
        }}
      />,
    )
    await waitFor(() => {
      expect(created.some((s) => (s.textContent || s.text || '').includes('__imgliInjectRan'))).toBe(true)
    })
    const live = document.body.querySelector('script[data-imgli-test-script]')
    expect(live).toBeTruthy()
    expect(live?.textContent).toContain('__imgliInjectRan')
    // 必须是我们 createElement 出来的节点，而非 template 里的死 script 克隆
    expect(created).toContain(live)
  } finally {
    spy.mockRestore()
  }
})

it('HtmlInject re-creates external script element with src/async', async () => {
  wrap(
    <HtmlInject
      inject={{
        head: '<script data-imgli-test-script src="https://example.com/analytics.js" async></script>',
        body_end: '',
      }}
    />,
  )
  await waitFor(() => {
    expect(document.head.querySelector('script[data-imgli-test-script][src="https://example.com/analytics.js"]')).toBeTruthy()
  })
  const s = document.head.querySelector('script[data-imgli-test-script]') as HTMLScriptElement
  expect(s.getAttribute('src')).toBe('https://example.com/analytics.js')
  expect(s.hasAttribute('async')).toBe(true)
})
