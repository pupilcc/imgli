import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button } from './Button'
import { Input } from './Input'
import { Segmented } from './Segmented'
import { Tag } from './Tag'
import { Toggle } from './Toggle'

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
  expect(screen.getByRole('button', { name: '登录' })).toHaveAttribute('aria-pressed', 'true')
  await userEvent.click(screen.getByRole('button', { name: '注册' }))
  expect(onChange).toHaveBeenCalledWith('reg')
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
