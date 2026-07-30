import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { QueueItem } from '../../upload/queue'
import { useUploadQueue } from '../../upload/queue'
import { useGlobal } from '../../store'
import { UploadCard } from './UploadCard'

function item(over: Partial<QueueItem>): QueueItem {
  return {
    id: 1, kind: 'file', name: 'shot.png', size: 1.8 * 1024 * 1024, ext: 'png',
    pct: 0, status: 'queued', thumb: null, reason: null, retryable: false,
    result: null, opts: { visibility: 'public', albumId: null, policyId: null, expiresIn: 0, maxViews: 0 },
    ...over,
  }
}

const LINKS = {
  url: 'https://img.li/i/k.png',
  markdown: '![shot.png](u)',
  html: '<img>',
  bbcode: '[img]u[/img]',
  thumbnail_url: 'https://img.li/t/k.jpg',
  share_url: 'https://img.li/s/k',
}

beforeEach(() => useGlobal.setState({ toasts: [] }))

it('uploading 显示进度条与百分比', () => {
  render(<UploadCard item={item({ status: 'uploading', pct: 34 })} />)
  expect(screen.getByText('上传中')).toBeInTheDocument()
  expect(screen.getByText('34%')).toBeInTheDocument()
})

it('instant 显示秒传徽章与说明', () => {
  render(<UploadCard item={item({ status: 'instant', pct: 100, result: { key: 'k', name: 'shot.png', size: 1, instant: true, links: LINKS } })} />)
  expect(screen.getByText('秒传')).toBeInTheDocument()
  expect(screen.getByText('已存在相同文件，直接返回链接')).toBeInTheDocument()
})

it('success 主链输入框 + 复制 URL / 多格式 / 分享页', async () => {
  const user = userEvent.setup()
  const writeText = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText },
    configurable: true,
  })
  render(
    <UploadCard
      item={item({
        status: 'success',
        pct: 100,
        result: { key: 'k', name: 'shot.png', size: 1, instant: false, links: LINKS },
      })}
    />,
  )
  const primary = screen.getByDisplayValue(LINKS.url)
  expect(primary).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '复制直链' }))
  expect(writeText).toHaveBeenCalledWith(LINKS.url)
  await user.click(screen.getByRole('button', { name: '复制分享页' }))
  expect(writeText).toHaveBeenCalledWith(LINKS.share_url)
  expect(screen.getByDisplayValue(LINKS.share_url)).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'MD' }))
  expect(writeText).toHaveBeenCalledWith(LINKS.markdown)
  await user.click(screen.getByRole('button', { name: '复制全部' }))
  expect(writeText).toHaveBeenCalledWith(
    [LINKS.url, LINKS.markdown, LINKS.html, LINKS.bbcode, LINKS.share_url].join('\n'),
  )
  const share = screen.getByRole('link', { name: /打开分享页/ })
  expect(share).toHaveAttribute('href', LINKS.share_url)
  expect(useGlobal.getState().toasts.length).toBeGreaterThan(0)
})

it('private 成功不显示分享页', () => {
  render(
    <UploadCard
      item={item({
        status: 'success',
        pct: 100,
        opts: { visibility: 'private', albumId: null, policyId: null, expiresIn: 0, maxViews: 0 },
        result: { key: 'k', name: 'shot.png', size: 1, instant: false, links: LINKS },
      })}
    />,
  )
  expect(screen.getByDisplayValue(LINKS.url)).toBeInTheDocument()
  expect(screen.queryByRole('link', { name: /打开分享页/ })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '复制分享页' })).not.toBeInTheDocument()
})

it('failed 显示原因，retryable 才有重试按钮', () => {
  const { rerender } = render(<UploadCard item={item({ status: 'failed', reason: '网络错误 — 连接超时', retryable: true })} />)
  expect(screen.getByText('失败')).toBeInTheDocument()
  expect(screen.getByText(/网络错误/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '重试' })).toBeInTheDocument()
  rerender(<UploadCard item={item({ status: 'failed', reason: '格式不允许', retryable: false })} />)
  expect(screen.queryByRole('button', { name: '重试' })).not.toBeInTheDocument()
})

it('重试与移除调 store 对应动作', async () => {
  const user = userEvent.setup()
  const retry = vi.spyOn(useUploadQueue.getState(), 'retry').mockImplementation(() => {})
  const remove = vi.spyOn(useUploadQueue.getState(), 'remove').mockImplementation(() => {})
  render(<UploadCard item={item({ id: 9, status: 'failed', reason: 'x', retryable: true })} />)
  await user.click(screen.getByRole('button', { name: '重试' }))
  expect(retry).toHaveBeenCalledWith(9)
  await user.click(screen.getByRole('button', { name: '移除' }))
  expect(remove).toHaveBeenCalledWith(9)
  retry.mockRestore()
  remove.mockRestore()
})
