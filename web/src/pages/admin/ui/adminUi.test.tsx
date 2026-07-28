import { fireEvent, render, screen } from '@testing-library/react'
import { AdminError } from './AdminError'
import { Pager } from './Pager'

it('Pager:边界禁用与回调', () => {
  const onPage = vi.fn()
  render(<Pager page={1} limit={50} total={120} onPage={onPage} />)
  expect(screen.getByText('第 1 / 3 页')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '‹ 上一页' })).toBeDisabled()
  fireEvent.click(screen.getByRole('button', { name: '下一页 ›' }))
  expect(onPage).toHaveBeenCalledWith(2)
})

it('Pager:单页时不渲染', () => {
  const { container } = render(<Pager page={1} limit={50} total={30} onPage={() => {}} />)
  expect(container.firstChild).toBeNull()
})

it('AdminError:重试回调', () => {
  const onRetry = vi.fn()
  render(<AdminError onRetry={onRetry} />)
  expect(screen.getByText('加载失败')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '重试' }))
  expect(onRetry).toHaveBeenCalled()
})
