import { useGlobal } from '../store'
import { copyText } from './copy'

beforeEach(() => useGlobal.setState({ toasts: [] }))

it('复制成功触发 toast', async () => {
  const writeText = vi.fn().mockResolvedValue(undefined)
  Object.assign(navigator, { clipboard: { writeText } })
  await copyText('https://img.li/i/abc.png', 'URL 链接')
  expect(writeText).toHaveBeenCalledWith('https://img.li/i/abc.png')
  expect(useGlobal.getState().toasts[0].message).toBe('已复制 URL 链接')
})

it('复制失败提示', async () => {
  Object.assign(navigator, { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } })
  await copyText('x', 'URL 链接')
  expect(useGlobal.getState().toasts[0].message).toBe('复制失败')
})
