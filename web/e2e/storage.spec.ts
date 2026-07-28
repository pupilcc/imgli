import { expect, test, type Page } from '@playwright/test'

// boss 由 admin.spec 首用例注册;本 spec 仅登录复用(workers:1 全套串行)。
const BOSS = { username: 'boss', password: 'bosspass-777' }
const POLICY_NAME = 'e2e-s3-policy'
const WEBDAV_POLICY_NAME = 'e2e-webdav-policy'
// 尾 4 固定,便于断言 maskAPIKey → "****1234"
const SECRET = 's3secret1234'
const MASKED = '****1234'
const DAV_PASSWORD = 'davsecretwxyz'
const DAV_MASKED = '****wxyz'

async function loginBoss(page: Page) {
  await page.goto('/login')
  await page.getByLabel('账号').fill(BOSS.username)
  await page.getByLabel('密码').fill(BOSS.password)
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()
}

/** 失败遗留时尽力经 API 删掉测试策略(独立行,删即净)。 */
async function cleanupPolicy(page: Page) {
  try {
    const res = await page.request.get('/api/v1/admin/policies')
    if (!res.ok()) return
    const body = (await res.json()) as { data?: { items?: { id: number; name: string }[] } }
    const names = new Set([POLICY_NAME, WEBDAV_POLICY_NAME])
    for (const p of body.data?.items ?? []) {
      if (names.has(p.name)) {
        await page.request.delete(`/api/v1/admin/policies/${p.id}`)
      }
    }
  } catch {
    /* ignore */
  }
}

test.afterEach(async ({ page }) => {
  await cleanupPolicy(page)
})

test('admin 建 s3 存储策略 UI 流:新建→掩码回显→删除', async ({ page }) => {
  await loginBoss(page)

  // 1) 存储策略页
  await page.goto('/admin/policies')
  await expect(page.getByRole('heading', { name: '存储策略管理' })).toBeVisible()

  // 2) 新建 → 选 S3 → 条件字段
  await page.getByRole('button', { name: /新建策略/ }).click()
  await page.getByRole('button', { name: 'S3', exact: true }).click()

  await expect(page.getByLabel('Endpoint')).toBeVisible()
  await expect(page.getByLabel('Region')).toBeVisible()
  await expect(page.getByLabel('Bucket')).toBeVisible()
  await expect(page.getByLabel('AccessKey ID')).toBeVisible()
  await expect(page.getByLabel('AccessKey Secret')).toBeVisible()
  await expect(page.getByLabel('存储路径')).toHaveCount(0)

  await page.getByLabel('名称').fill(POLICY_NAME)
  await page.getByLabel('Endpoint').fill('s3.us-east-1.amazonaws.com')
  await page.getByLabel('Region').fill('us-east-1')
  await page.getByLabel('Bucket').fill('e2e-bucket')
  await page.getByLabel('AccessKey ID').fill('AKIAe2eTEST')
  await page.getByLabel('AccessKey Secret').fill(SECRET)
  // 仅写库,不测连接;CreatePolicy 字段非空即通过
  await page.getByRole('button', { name: '保存' }).click()

  // 列表出现该策略
  const row = page.getByRole('button', { name: new RegExp(POLICY_NAME) })
  await expect(row).toBeVisible()

  // 3) 编辑 → 驱动只读 S3 + secret 打码
  await row.click()
  await expect(page.locator('[class*=driver]')).toHaveText('S3')
  await expect(page.getByLabel('AccessKey Secret')).toHaveValue(MASKED)
  await expect(page.getByText(/已设密钥显示为掩码/)).toBeVisible()

  // 4) 删除(InlineConfirm 两击)→ 列表移除(空策略无 files 引用,可删)
  await page.getByRole('button', { name: '删除' }).click()
  await page.getByRole('button', { name: '确认删除？' }).click()
  await expect(page.getByRole('button', { name: new RegExp(POLICY_NAME) })).toHaveCount(0)
})

test('admin 建 webdav 存储策略 UI 流:新建→掩码回显→删除', async ({ page }) => {
  await loginBoss(page)

  // 1) 存储策略页
  await page.goto('/admin/policies')
  await expect(page.getByRole('heading', { name: '存储策略管理' })).toBeVisible()

  // 2) 新建 → 选 WebDAV → 条件字段
  await page.getByRole('button', { name: /新建策略/ }).click()
  await page.getByRole('button', { name: 'WebDAV', exact: true }).click()

  await expect(page.getByLabel('Endpoint')).toBeVisible()
  await expect(page.getByLabel('用户名')).toBeVisible()
  await expect(page.getByLabel('密码')).toBeVisible()
  await expect(page.getByLabel('存储路径')).toHaveCount(0)
  await expect(page.getByLabel('Region')).toHaveCount(0)
  await expect(page.getByLabel('Bucket')).toHaveCount(0)
  await expect(page.getByLabel('AccessKey Secret')).toHaveCount(0)

  await page.getByLabel('名称').fill(WEBDAV_POLICY_NAME)
  await page.getByLabel('Endpoint').fill('https://dav.example.test/imgli')
  await page.getByLabel('用户名').fill('davuser')
  await page.getByLabel('密码').fill(DAV_PASSWORD)
  // 仅写库,不真连;CreatePolicy 字段非空即通过
  await page.getByRole('button', { name: '保存' }).click()

  // 列表出现该策略
  const row = page.getByRole('button', { name: new RegExp(WEBDAV_POLICY_NAME) })
  await expect(row).toBeVisible()

  // 3) 编辑 → 驱动只读 WebDAV + password 打码
  await row.click()
  await expect(page.locator('[class*=driver]')).toHaveText('WebDAV')
  await expect(page.getByLabel('密码')).toHaveValue(DAV_MASKED)
  await expect(page.getByText(/已设密钥显示为掩码/)).toBeVisible()

  // 4) 删除(InlineConfirm 两击)→ 列表移除(空策略无 files 引用,可删)
  await page.getByRole('button', { name: '删除' }).click()
  await page.getByRole('button', { name: '确认删除？' }).click()
  await expect(page.getByRole('button', { name: new RegExp(WEBDAV_POLICY_NAME) })).toHaveCount(0)
})
