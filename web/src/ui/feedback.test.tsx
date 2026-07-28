import { act, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useGlobal } from '../store'
import { InlineConfirm } from './InlineConfirm'
import { Modal } from './Modal'
import { Toasts } from './Toasts'

beforeEach(() => useGlobal.setState({ toasts: [] }))

it('Toasts 渲染队列并随 store 移除', () => {
  render(<Toasts />)
  act(() => useGlobal.getState().pushToast('已复制 URL 链接'))
  expect(screen.getByText('已复制 URL 链接')).toBeInTheDocument()
  act(() => useGlobal.setState({ toasts: [] }))
  expect(screen.queryByText('已复制 URL 链接')).not.toBeInTheDocument()
})

it('Modal 遮罩点击与 Escape 关闭，内容点击不关', async () => {
  const onClose = vi.fn()
  render(
    <Modal open onClose={onClose}>
      <p>弹窗内容</p>
    </Modal>,
  )
  await userEvent.click(screen.getByText('弹窗内容'))
  expect(onClose).not.toHaveBeenCalled()
  await userEvent.keyboard('{Escape}')
  expect(onClose).toHaveBeenCalledOnce()
  await userEvent.click(screen.getByRole('dialog').parentElement!)
  expect(onClose).toHaveBeenCalledTimes(2)
})

it('InlineConfirm 两击确认、超时还原', async () => {
  vi.useFakeTimers()
  const onConfirm = vi.fn()
  render(<InlineConfirm label="删除" onConfirm={onConfirm} />)

  fireEvent.click(screen.getByRole('button', { name: '删除' }))
  expect(onConfirm).not.toHaveBeenCalled()
  expect(screen.getByRole('button', { name: '确认删除？' })).toBeInTheDocument()

  act(() => vi.advanceTimersByTime(2600))
  expect(screen.getByRole('button', { name: '删除' })).toBeInTheDocument()

  fireEvent.click(screen.getByRole('button', { name: '删除' }))
  fireEvent.click(screen.getByRole('button', { name: '确认删除？' }))
  expect(onConfirm).toHaveBeenCalledOnce()
  expect(screen.getByRole('button', { name: '删除' })).toBeInTheDocument()
  vi.useRealTimers()
})
