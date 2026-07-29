import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { EmptyState } from './EmptyState'
import { QuotaAlertBar } from './QuotaAlertBar'
import { QuotaBar, quotaLevel } from './QuotaBar'
import { Skeleton } from './Skeleton'

const GB = 1024 ** 3

it('quotaLevel 阈值', () => {
  expect(quotaLevel(0, 10 * GB)).toBe('ok')
  expect(quotaLevel(7.9 * GB, 10 * GB)).toBe('ok')
  expect(quotaLevel(8 * GB, 10 * GB)).toBe('warn')
  expect(quotaLevel(10 * GB, 10 * GB)).toBe('full')
  expect(quotaLevel(1, 0)).toBe('ok')
})

it('QuotaBar 显示用量数字', () => {
  render(
    <MemoryRouter>
      <QuotaBar used={2.14 * GB} total={10 * GB} kind="storage" />
    </MemoryRouter>,
  )
  expect(screen.getByText('STORAGE')).toBeInTheDocument()
  expect(screen.getByText('2.14 / 10 GB')).toBeInTheDocument()
})

it('QuotaAlertBar 三态', () => {
  const { rerender, container } = render(
    <MemoryRouter>
      <QuotaAlertBar used={1 * GB} total={10 * GB} />
    </MemoryRouter>,
  )
  expect(container.textContent).toBe('')
  rerender(
    <MemoryRouter>
      <QuotaAlertBar used={8.4 * GB} total={10 * GB} />
    </MemoryRouter>,
  )
  expect(screen.getByText(/容量已使用 84%/)).toBeInTheDocument()
  expect(screen.getByText('管理容量 →')).toHaveAttribute('href', '/settings')
  rerender(
    <MemoryRouter>
      <QuotaAlertBar used={10 * GB} total={10 * GB} />
    </MemoryRouter>,
  )
  expect(screen.getByText(/容量已满/)).toBeInTheDocument()
})

it('EmptyState 与 Skeleton 渲染', () => {
  render(
    <EmptyState title="还没有图片" desc="上传第一张图片，即刻获得外链">
      <button>去上传 →</button>
    </EmptyState>,
  )
  expect(screen.getByText('EMPTY')).toBeInTheDocument()
  expect(screen.getByText('还没有图片')).toBeInTheDocument()
  const { container } = render(<Skeleton height={120} />)
  expect(container.firstChild).toHaveStyle({ height: '120px' })
})
