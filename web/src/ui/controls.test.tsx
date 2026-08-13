import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ArmedButton } from './ArmedButton'
import { Button } from './Button'
import { Input } from './Input'
import { Segmented } from './Segmented'
import { Tag } from './Tag'
import { Toggle } from './Toggle'

it('ArmedButton 第一击 armed 显示 armedChildren，第二击才 onConfirm', async () => {
  const onConfirm = vi.fn()
  render(
    <ArmedButton title="删除" armedTitle="确认删除" armedChildren="确认" onConfirm={onConfirm}>
      ×
    </ArmedButton>,
  )
  const btn = screen.getByRole('button', { name: '删除' })
  expect(btn).toHaveTextContent('×')
  await userEvent.click(btn)
  expect(onConfirm).not.toHaveBeenCalled()
  expect(screen.getByRole('button', { name: '确认删除' })).toHaveTextContent('确认')
  await userEvent.click(screen.getByRole('button', { name: '确认删除' }))
  expect(onConfirm).toHaveBeenCalledOnce()
})

it('Button 变体渲染并可点击', async () => {
  const onClick = vi.fn()
  render(<Button variant="primary" onClick={onClick}>上传</Button>)
  await userEvent.click(screen.getByRole('button', { name: '上传' }))
  expect(onClick).toHaveBeenCalledOnce()
})

it('Segmented 高亮当前值并回调', async () => {
  const onChange = vi.fn()
  render(
    <Segmented
      options={[
        { value: 'login', label: '登录' },
        { value: 'reg', label: '注册' },
      ]}
      value="login"
      onChange={onChange}
    />,
  )
  const login = screen.getByRole('button', { name: '登录' })
  expect(login).toHaveAttribute('aria-pressed', 'true')
  // 选中态必须直接写 CSS 变量色，避免工具类冲突导致「未 hover 看不见字」
  expect(login).toHaveStyle({ backgroundColor: 'var(--btn)', color: 'var(--btnText)' })
  const reg = screen.getByRole('button', { name: '注册' })
  expect(reg).not.toHaveStyle({ color: 'var(--btnText)' })
  await userEvent.click(reg)
  expect(onChange).toHaveBeenCalledWith('reg')
})

it('Segmented mono 选中项保留反色字色类', () => {
  // mono 会带上自定义字号 text-xs-plus；曾因 tailwind-merge 误判为颜色类而吃掉
  // text-btn-text / text-muted，导致上传选项「有效期/访问次数」选中项看不见字。
  render(
    <Segmented
      mono
      options={[
        { value: 'never', label: '永久' },
        { value: '7d', label: '7 天' },
      ]}
      value="never"
      onChange={vi.fn()}
    />,
  )
  const active = screen.getByRole('button', { name: '永久' })
  expect(active.className).toContain('text-btn-text')
  expect(active.className).toContain('text-xs-plus')
  const inactive = screen.getByRole('button', { name: '7 天' })
  expect(inactive.className).toContain('text-muted')
  expect(inactive.className).toContain('text-xs-plus')
})

it('Input 渲染 label 并透传属性', () => {
  render(<Input label="邮箱" placeholder="you@example.com" />)
  expect(screen.getByLabelText('邮箱')).toHaveAttribute('placeholder', 'you@example.com')
})

it('Toggle 是 switch 且可切换', async () => {
  const onChange = vi.fn()
  render(<Toggle checked={false} onChange={onChange} />)
  const sw = screen.getByRole('switch')
  expect(sw).toHaveAttribute('aria-checked', 'false')
  await userEvent.click(sw)
  expect(onChange).toHaveBeenCalledWith(true)
})

it('Tag 渲染文本', () => {
  render(<Tag variant="inverse">秒传</Tag>)
  expect(screen.getByText('秒传')).toBeInTheDocument()
})
