import { expect, test, type Page } from '@playwright/test'

// Playwright 默认 navigator.language=en-US → detectLang 返 en;强制 zh-CN 首屏中文再测切换
test.use({ locale: 'zh-CN' })

/** 登录 boss；单独跑 lang.spec 时无 admin 前置，则注册为首个管理员。 */
async function loginBoss(page: Page) {
  await page.goto('/login')
  await page.getByLabel('账号').fill('boss')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  const drop = page.getByTestId('dropzone')
  // 登录成功后 AuthPage 延迟 400ms 再 navigate，须等待 dropzone，勿立即判失败
  try {
    await expect(drop).toBeVisible({ timeout: 8_000 })
    return
  } catch {
    // 登录失败（用户不存在）→ 注册为首管理员
  }
  await page.getByRole('button', { name: '注册' }).click()
  await page.getByLabel('用户名').fill('boss')
  await page.getByLabel('邮箱').fill('boss@img.li')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  await expect(drop).toBeVisible()
}

/** LangToggle: aria-label=language, 文案 zh 态 "EN" / en 态 "中" */
function langToggle(page: Page) {
  return page.getByRole('button', { name: 'language' })
}

test.afterEach(async ({ page }) => {
  // 还原:清 localStorage lang,防影响其它 spec
  await page.evaluate(() => localStorage.removeItem('imgli-lang')).catch(() => {})
})

test('语言切换:登录页 zh↔en 文案变+刷新持久', async ({ page }) => {
  await page.goto('/login')
  // 首屏 zh(locale zh-CN):中文登录页文案
  await expect(page.getByRole('heading', { name: '欢迎回来' })).toBeVisible()
  await expect(langToggle(page)).toHaveText('EN')

  // 点 LangToggle → 切英文
  await langToggle(page).click()
  await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible()
  expect(await page.evaluate(() => localStorage.getItem('imgli-lang'))).toBe('en')
  // setHtmlLang: en → lang="en"
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
  await expect(langToggle(page)).toHaveText('中')

  // 刷新持久(imgli-lang=en)
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
})

test('登录同步:切 en 写偏好→清 localStorage 换设备→登录应用 en', async ({ page }) => {
  await loginBoss(page)
  // 登录态切 en(LangToggle 在 Nav)→ 写 Preferences.lang
  await expect(langToggle(page)).toHaveText('EN')
  await langToggle(page).click()
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
  await expect(langToggle(page)).toHaveText('中')
  // 等偏好 PATCH 落库
  await page.waitForTimeout(300)

  // 模拟换设备:清 localStorage(去掉本地 lang) + 清 cookie 登出
  await page.evaluate(() => localStorage.removeItem('imgli-lang'))
  await page.context().clearCookies()

  // 重新登录 → App 读 session.preferences.lang=en 应用
  // 登录页仍 zh(locale zh-CN,无本地 lang),标签仍中文
  await loginBoss(page)
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')

  // 还原:切回 zh(写偏好 zh,防污染 boss 后续 spec)
  await langToggle(page).click()
  // setHtmlLang: zh → lang="zh-CN"
  await expect(page.locator('html')).toHaveAttribute('lang', 'zh-CN')
  await page.waitForTimeout(300)
})
