import { expect, test } from '@playwright/test'

// 1x1 透明 PNG——字节与 admin.spec/main.spec 的 PNG 均不同,避免撞秒传去重
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=',
  'base64',
)

async function loginBoss(page: import('@playwright/test').Page) {
  await page.goto('/login')
  await page.getByLabel('账号').fill('boss')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()
}

async function setGuestSwitch(page: import('@playwright/test').Page, on: boolean) {
  await page.goto('/admin/settings')
  const sw = page.getByRole('switch', { name: '允许游客上传' })
  await expect(sw).toHaveAttribute('aria-checked', String(!on))
  await sw.click()
  await expect(sw).toHaveAttribute('aria-checked', String(on))
  await page.getByRole('button', { name: '保存设置' }).click()
  await expect(page.getByText('已保存')).toBeVisible()
}

test('游客上传：开开关→匿名上传拿链接→关开关→匿名跳登录', async ({ page }) => {
  // admin 开启游客上传
  await loginBoss(page)
  await setGuestSwitch(page, true)

  // 匿名访 / → 游客上传页(无导航),上传 → 复制链接
  await page.context().clearCookies()
  await page.goto('/')
  await expect(page.getByTestId('dropzone')).toBeVisible()
  await expect(page.getByText('游客模式')).toBeVisible()
  await expect(page.getByRole('link', { name: '登录以管理图片' })).toBeVisible()
  await expect(page.getByRole('link', { name: '我的图片' })).toHaveCount(0)
  const chooser = page.waitForEvent('filechooser')
  await page.getByTestId('dropzone').click()
  await (await chooser).setFiles({ name: 'guest-shot.png', mimeType: 'image/png', buffer: PNG })
  await expect(page.getByText('已完成', { exact: true })).toBeVisible()
  // UploadCard 主按钮文案「复制直链」（不再有 exact name=URL 的格式钮）
  await page.getByRole('button', { name: '复制直链' }).click()
  await expect(page.getByText(/已复制 直链 URL/)).toBeVisible()

  // admin 关回开关
  await loginBoss(page)
  await setGuestSwitch(page, false)

  // 匿名再访 / → 仍在上传落地页，展示登录门（非硬跳 /login）
  await page.context().clearCookies()
  await page.goto('/')
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByTestId('login-gate')).toBeVisible()
  await expect(page.getByRole('link', { name: /登录/ })).toBeVisible()
})
