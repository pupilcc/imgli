import { expect, test, type Page } from '@playwright/test'

// 1x1 绿色 PNG——字节与 admin/main/guest/settings 均不同，避免秒传去重撞 hash
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+P+/HgAFhAJ/wlseKgAAAABJRU5ErkJggg==',
  'base64',
)

/** 登录 boss；单独跑 plaza.spec 时无 admin 前置，则注册为首个管理员。 */
async function loginBoss(page: Page) {
  await page.goto('/login')
  await page.getByLabel('账号').fill('boss')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  const drop = page.getByTestId('dropzone')
  if (await drop.isVisible().catch(() => false)) return
  // 登录失败（用户不存在）→ 注册为首管理员
  await page.getByRole('button', { name: '注册' }).click()
  await page.getByLabel('用户名').fill('boss')
  await page.getByLabel('邮箱').fill('boss@img.li')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  await expect(drop).toBeVisible()
}

/** admin 系统设置「启用广场」；已是目标态则跳过点击（afterEach 幂等）。 */
async function setPlazaSwitch(page: Page, on: boolean) {
  await page.goto('/admin/settings')
  const sw = page.getByRole('switch', { name: '启用广场' })
  await expect(sw).toBeVisible()
  const cur = await sw.getAttribute('aria-checked')
  if (cur !== String(on)) {
    await sw.click()
    await expect(sw).toHaveAttribute('aria-checked', String(on))
    await page.getByRole('button', { name: '保存设置' }).click()
    await expect(page.getByText('已保存')).toBeVisible()
  }
}

/** 资料页「公开主页」；Toggle 点击后立即 mutate，等 toast。已是目标态则跳过。 */
async function setPublicProfile(page: Page, on: boolean) {
  await page.goto('/settings/profile')
  const sw = page.getByRole('switch', { name: '公开主页' })
  await expect(sw).toBeVisible()
  const cur = await sw.getAttribute('aria-checked')
  if (cur !== String(on)) {
    await sw.click()
    await expect(sw).toHaveAttribute('aria-checked', String(on))
    await expect(page.getByText('已保存')).toBeVisible()
  }
}

test.afterEach(async ({ page }) => {
  try {
    await loginBoss(page)
    await setPublicProfile(page, false)
    await setPlazaSwitch(page, false)
  } catch {
    /* 尽力还原，不因还原失败再抛 */
  }
})

test('广场：admin 开→用户开主页+传公开图→匿名浏览/explore 与 /u/→排序→关→关闭态', async ({ page }) => {
  await loginBoss(page)
  await setPlazaSwitch(page, true)
  await setPublicProfile(page, true)

  // 传一张唯一公开图（默认可见性 public）
  await page.goto('/')
  const chooser = page.waitForEvent('filechooser')
  await page.getByTestId('dropzone').click()
  await (await chooser).setFiles({ name: 'plaza-shot.png', mimeType: 'image/png', buffer: PNG })
  await expect(page.getByText('已完成', { exact: true })).toBeVisible()

  // 匿名浏览广场
  await page.context().clearCookies()
  await page.goto('/explore')
  await expect(page.getByRole('heading', { name: '广场' })).toBeVisible()
  // ImageCard 作者：nickname || username；boss 无昵称时显示 boss
  await expect(page.getByRole('link', { name: /boss/i }).first()).toBeVisible()

  // 点作者 → /u/boss 主页
  await page.getByRole('link', { name: /boss/i }).first().click()
  await expect(page).toHaveURL(/\/u\/boss$/)
  await expect(page.getByText('@boss')).toBeVisible()

  // 排序切换不崩（回 /explore 切热门）
  await page.goto('/explore')
  await page.getByRole('button', { name: '热门' }).click()
  await expect(page.getByRole('link', { name: /boss/i }).first()).toBeVisible()

  // admin 关广场 → 匿名见关闭态
  await loginBoss(page)
  await setPlazaSwitch(page, false)
  // 服务端应已 404（APIRequestContext 不走浏览器磁盘缓存）
  await expect
    .poll(async () => (await page.request.get('/api/v1/plaza?sort=new&limit=1')).status())
    .toBe(404)
  // GET /plaza 带 Cache-Control: public, max-age=30；页面 fetch 会命中开启态 200 缓存。
  // 用 route 改走 page.request，绕过浏览器缓存。
  await page.route('**/api/v1/plaza**', async (route) => {
    const res = await page.request.fetch(route.request())
    await route.fulfill({
      status: res.status(),
      headers: { ...res.headers(), 'cache-control': 'no-store' },
      body: await res.body(),
    })
  })
  await page.context().clearCookies()
  await page.goto('/explore')
  await expect(page.getByText('广场未开启')).toBeVisible()
})
