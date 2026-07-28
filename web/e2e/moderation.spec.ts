import { expect, test, type Browser, type Page } from '@playwright/test'

const BOSS = { username: 'boss', password: 'bosspass-777' }

test.describe.configure({ mode: 'serial' })

/** 测试中若改过机审配置且未还原,afterEach 必须兜底。 */
let moderationDirty = false

async function login(page: Page, account: string, password: string) {
  await page.goto('/login')
  await page.getByLabel('账号').fill(account)
  await page.getByLabel('密码').fill(password)
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()
}

/** API 部分 PUT 还原默认机审(webhook + 关闭)。 */
async function putModerationDefault(page: Page) {
  const res = await page.request.put('/api/v1/admin/settings', {
    data: {
      moderation: {
        enabled: false,
        provider: 'webhook',
        endpoint: '',
        api_key: '',
        access_key_id: '',
        access_key_secret: '',
        region: '',
        threshold: 0.8,
        action: 'pending',
      },
    },
  })
  expect(res.ok()).toBeTruthy()
}

async function restoreModerationViaAPI(browser: Browser) {
  const ctx = await browser.newContext()
  const page = await ctx.newPage()
  try {
    await login(page, BOSS.username, BOSS.password)
    await putModerationDefault(page)
    moderationDirty = false
  } finally {
    await ctx.close()
  }
}

test.afterEach(async ({ browser }) => {
  if (!moderationDirty) return
  await restoreModerationViaAPI(browser)
})

test('机器审核:provider 切换条件渲染 + 还原', async ({ page }) => {
  await login(page, BOSS.username, BOSS.password)
  await page.goto('/admin/settings')
  await expect(page.getByRole('heading', { name: '机器审核' })).toBeVisible()

  // 默认 webhook:Webhook 地址 + API Key 可见
  await expect(page.getByLabel('Webhook 地址')).toBeVisible()
  await expect(page.getByLabel('API Key')).toBeVisible()

  // dirty 须在改动 UI 之前置位:失败时 afterEach 才不会漏还原
  moderationDirty = true

  // 切 OpenAI → API Key 可见、Webhook 地址不可见
  await page.getByRole('button', { name: 'OpenAI' }).click()
  await expect(page.getByLabel('API Key')).toBeVisible()
  await expect(page.getByLabel('Webhook 地址')).toHaveCount(0)

  // 切 aliyun → Region 可见
  await page.getByRole('button', { name: '阿里云' }).click()
  await expect(page.getByLabel('Region')).toBeVisible()
  await expect(page.getByLabel('AccessKey ID')).toBeVisible()

  // 成功路径就地还原:切回 webhook + API 关闭 enabled
  await page.getByRole('button', { name: 'Webhook' }).click()
  await putModerationDefault(page)
  moderationDirty = false
})
