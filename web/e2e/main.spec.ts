import { expect, test } from '@playwright/test'

// 1x1 红色 PNG
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
  'base64',
)

test('主干：注册→上传→复制→删除→回收站恢复→彻底删除', async ({ page }) => {
  // 注册（本套件第三个用户——admin.spec 已先注册 boss/pleb，故非首管理员）
  // guest 关闭时 / 仍是上传落地页（非硬跳登录）；注册走 /login
  await page.goto('/login')
  await page.getByRole('button', { name: '注册' }).click()
  await page.getByLabel('用户名').fill('e2e')
  await page.getByLabel('邮箱').fill('e2e@img.li')
  await page.getByLabel('密码').fill('e2epass-777')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  // 上传
  const chooser = page.waitForEvent('filechooser')
  await page.getByTestId('dropzone').click()
  await (await chooser).setFiles({ name: 'e2e-shot.png', mimeType: 'image/png', buffer: PNG })
  await expect(page.getByText('已完成', { exact: true })).toBeVisible()

  // 复制链接（断言 toast，headless 剪贴板权限不稳）
  await page.getByRole('button', { name: 'MD', exact: true }).click()
  await expect(page.getByText(/已复制 MD/)).toBeVisible()

  // 图库删除（hover 两击）
  await page.getByRole('link', { name: '我的图片' }).click()
  await expect(page.getByText('e2e-shot.png')).toBeVisible()
  const card = page.locator('main [class*=card]').first()
  await card.hover()
  await card.getByTitle('移入回收站').click()
  await card.getByTitle('确认移入回收站').click()
  await expect(page.getByText('还没有图片')).toBeVisible()

  // 回收站恢复
  await page.goto('/trash')
  await expect(page.getByText('e2e-shot.png')).toBeVisible()
  await page.getByRole('button', { name: '恢复' }).click()
  await expect(page.getByText(/已恢复/)).toBeVisible()
  await page.goto('/images')
  await expect(page.getByText('e2e-shot.png')).toBeVisible()

  // 再删 → 彻底删除 → 空态
  const card2 = page.locator('main [class*=card]').first()
  await card2.hover()
  await card2.getByTitle('删除').click()
  await card2.getByTitle('确认删除').click()
  await page.goto('/trash')
  await page.getByRole('button', { name: '彻底删除' }).click()
  await page.getByRole('button', { name: '确认删除？' }).click()
  await expect(page.getByText('回收站是空的')).toBeVisible()
})
