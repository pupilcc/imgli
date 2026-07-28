import { expect, test, type Browser, type BrowserContext, type Page } from '@playwright/test'

// 与 admin/main/guest/settings 的 1x1 PNG 字节均不同,避免秒传串扰
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPj/HwADBwIAMCbHYQAAAABJRU5ErkJggg==',
  'base64',
)

const D1HOT = { username: 'd1hot', email: 'd1hot@img.li', password: 'd1hotpass-777' }
const BOSS = { username: 'boss', password: 'bosspass-777' }

test.describe.configure({ mode: 'serial' })

/** 测试中若已开启防盗链且未还原,afterEach 必须兜底;成功路径在测完拦截后立即还原,避免再烧一次 auth 额度。 */
let hotlinkDirty = false

async function login(page: Page, account: string, password: string) {
  await page.goto('/login')
  await page.getByLabel('账号').fill(account)
  await page.getByLabel('密码').fill(password)
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()
}

/** API 部分 PUT 还原默认防盗链(与 settings 面一致;仅 afterEach/就地还原用,省 UI 导航)。 */
async function putHotlinkDefault(page: Page) {
  const res = await page.request.put('/api/v1/admin/settings', {
    data: { hotlink: { enabled: false, allowed_domains: [], allow_empty_referer: true } },
  })
  expect(res.ok()).toBeTruthy()
}

async function restoreHotlinkViaAPI(browser: Browser) {
  const ctx = await browser.newContext()
  const page = await ctx.newPage()
  try {
    await login(page, BOSS.username, BOSS.password)
    await putHotlinkDefault(page)
    hotlinkDirty = false
  } finally {
    await ctx.close()
  }
}

test.afterEach(async ({ browser }) => {
  if (!hotlinkDirty) return
  await restoreHotlinkViaAPI(browser)
})

test('防盗链全链:未开放行→开启拦截→详情 ACCESS', async ({ page, request, browser }) => {
  // 1) 注册 d1hot → 上传 → 取直链;evil Referer 默认放行 200
  await page.goto('/')
  await expect(page).toHaveURL(/\/login$/)
  await page.getByRole('button', { name: '注册' }).click()
  await page.getByLabel('用户名').fill(D1HOT.username)
  await page.getByLabel('邮箱').fill(D1HOT.email)
  await page.getByLabel('密码').fill(D1HOT.password)
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  const chooser = page.waitForEvent('filechooser')
  await page.getByTestId('dropzone').click()
  await (await chooser).setFiles({ name: 'd1hot-shot.png', mimeType: 'image/png', buffer: PNG })
  await expect(page.getByText('已完成', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'URL', exact: true }).click()
  await expect(page.getByText(/已复制 URL/)).toBeVisible()
  const directUrl = await page.evaluate(() => navigator.clipboard.readText())
  expect(directUrl).toMatch(/^https?:\/\//)

  // 保留 d1hot 会话,后续 ACCESS 用 storageState 复用,省一次 login 占用 auth 限速桶(20/min 与 register 共用)
  const d1hotState = await page.context().storageState()

  const openRes = await request.get(directUrl, { headers: { Referer: 'https://evil.example/x' } })
  expect(openRes.status()).toBe(200)

  // 2) admin 进系统设置开启防盗链
  let adminCtx: BrowserContext | null = await browser.newContext()
  try {
    const adminPage = await adminCtx.newPage()
    await login(adminPage, BOSS.username, BOSS.password)
    await adminPage.goto('/admin/settings')
    await expect(adminPage.getByRole('heading', { name: '防盗链' })).toBeVisible()

    const enabled = adminPage.getByRole('switch', { name: '启用防盗链' })
    if ((await enabled.getAttribute('aria-checked')) !== 'true') {
      await enabled.click()
    }
    await adminPage.getByLabel('允许的来源域名').fill('allowed.example')
    const allowEmpty = adminPage.getByRole('switch', { name: '允许空 Referer' })
    if ((await allowEmpty.getAttribute('aria-checked')) === 'true') {
      await allowEmpty.click()
    }
    // dirty 须在触发保存之前置位:服务端已落库而 UI 断言失败时,afterEach 才不会漏还原
    // (codex 终审)。还原成功处才清位。
    hotlinkDirty = true
    await adminPage.getByRole('button', { name: '保存设置' }).click()
    await expect(adminPage.getByText('已保存')).toBeVisible()

    const evil = await request.get(directUrl, { headers: { Referer: 'https://evil.example/x' } })
    expect(evil.status()).toBe(403)

    const allowed = await request.get(directUrl, { headers: { Referer: 'https://allowed.example/p' } })
    expect(allowed.status()).toBe(200)

    const noRef = await request.get(directUrl)
    expect(noRef.status()).toBe(403)

    // 成功路径就地还原,afterEach 可跳过
    await putHotlinkDefault(adminPage)
    hotlinkDirty = false
  } finally {
    await adminCtx.close()
    adminCtx = null
  }

  // 3) d1hot 详情弹窗 ACCESS 区块可见(计数值不断言——刷盘异步)
  const userCtx = await browser.newContext({
    storageState: d1hotState,
    permissions: ['clipboard-read', 'clipboard-write'],
  })
  try {
    const userPage = await userCtx.newPage()
    await userPage.goto('/images')
    await expect(userPage.getByText('d1hot-shot.png')).toBeVisible()
    await userPage.locator('main [class*=card]').first().click()
    await expect(userPage.getByRole('dialog')).toBeVisible()
    await expect(userPage.getByText(/ACCESS/)).toBeVisible()
    await expect(userPage.getByText(/总访问/)).toBeVisible()
  } finally {
    await userCtx.close()
  }
})
