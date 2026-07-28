import { expect, test } from '@playwright/test'

// 依赖 admin.spec 已注册 boss。结束必须把注册模式改回开放——main.spec 要注册 e2e 用户。
// 失败路径也须还原，故放 afterEach 无条件执行。
test.afterEach(async ({ page }) => {
  await page.context().clearCookies()
  await page.goto('/login')
  await page.getByLabel('账号').fill('boss')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()
  await page.goto('/admin/settings')
  const openBtn = page.getByRole('button', { name: '开放注册' })
  if ((await openBtn.getAttribute('aria-pressed')) !== 'true') {
    await openBtn.click()
    await page.getByRole('button', { name: '保存设置' }).click()
    await expect(page.getByText('已保存')).toBeVisible()
  }
})

test('邀请注册：admin 开邀请模式发码→匿名凭码注册→改回开放', async ({ page }) => {
  // boss 登录
  await page.goto('/login')
  await page.getByLabel('账号').fill('boss')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  // 切邀请模式
  await page.goto('/admin/settings')
  await page.getByRole('button', { name: '邀请注册' }).click()
  await page.getByRole('button', { name: '保存设置' }).click()
  await expect(page.getByText('已保存')).toBeVisible()

  // 生成 1 张码,从 Modal 抓明文
  await page.goto('/admin/invites')
  await page.getByRole('button', { name: '生成邀请码' }).click()
  await page.getByLabel('数量').fill('1')
  await page.getByRole('button', { name: '生成', exact: true }).click()
  const codeText = await page.locator('pre').textContent()
  const code = (codeText ?? '').trim()
  expect(code).toMatch(/^IL-[A-Z2-9]{4}-[A-Z2-9]{4}$/)
  await page.getByRole('button', { name: '完成' }).click()
  // 列表出现这张未使用码
  await expect(page.getByText(code)).toBeVisible()

  // 匿名凭码注册
  await page.context().clearCookies()
  await page.goto('/login')
  await page.getByRole('button', { name: '注册' }).click()
  await page.getByLabel('用户名').fill('invitee')
  await page.getByLabel('邮箱').fill('invitee@img.li')
  await page.getByLabel('密码').fill('invitepass-1')
  await page.getByLabel('邀请码').fill(code)
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  // boss 复核:码已核销（改回开放注册由 afterEach 无条件执行）
  await page.context().clearCookies()
  await page.goto('/login')
  await page.getByLabel('账号').fill('boss')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()
  await page.goto('/admin/invites?status=used')
  await expect(page.getByText(code)).toBeVisible()
  await expect(page.getByText('invitee')).toBeVisible()
})
