import { expect, test, type Browser, type BrowserContext, type Page } from '@playwright/test'

// 与 admin/main/guest/settings/hotlink 的 PNG 字节均互异,避免秒传串扰
// 3×2 / 4×4 / 2×3 程序生成的 RGBA PNG
const PNG_A = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAYAAACddGYaAAAAEUlEQVR4nGPgEpH7D8MMyBwAV1kHYzmDy0UAAAAASUVORK5CYII=',
  'base64',
)
const PNG_WM = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAQAAAAECAYAAACp8Z5+AAAAEklEQVR4nGP4z9BwAhkzkC4AANiIJHFbP1qpAAAAAElFTkSuQmCC',
  'base64',
)
const PNG_B = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAIAAAADCAYAAAC56t6BAAAAEUlEQVR4nGNgZGL+D8IMGAwASb8GHwDKgi0AAAAASUVORK5CYII=',
  'base64',
)

const D2PROC = { username: 'd2proc', email: 'd2proc@img.li', password: 'd2procpass-777' }
const BOSS = { username: 'boss', password: 'bosspass-777' }

const PROCESSING_DEFAULT = {
  text_watermark: { enabled: false, text: '', position: 'br', opacity: 0.35, size_ratio: 0.04 },
  max_edge: 0,
}

const PROCESSING_ON = {
  text_watermark: {
    enabled: true,
    text: 'imgli-e2e',
    position: 'br',
    opacity: 0.35,
    size_ratio: 0.04,
  },
  max_edge: 0,
}

test.describe.configure({ mode: 'serial' })

/** 测试中若已改 processing 且未还原,afterEach 必须兜底。 */
let processingDirty = false

// 复用 storageState 省 login/register,避再次消耗 auth 桶
let d2procState: Awaited<ReturnType<BrowserContext['storageState']>> | null = null
let bossState: Awaited<ReturnType<BrowserContext['storageState']>> | null = null

async function login(page: Page, account: string, password: string) {
  await page.goto('/login')
  await page.getByLabel('账号').fill(account)
  await page.getByLabel('密码').fill(password)
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()
}

/** API 部分 PUT 还原默认 processing(与 model 播种一致;仅 afterEach/就地还原用)。 */
async function putProcessingDefault(page: Page) {
  const res = await page.request.put('/api/v1/admin/settings', {
    data: { processing: PROCESSING_DEFAULT },
  })
  expect(res.ok()).toBeTruthy()
}

async function putProcessingOn(page: Page) {
  const res = await page.request.put('/api/v1/admin/settings', {
    data: { processing: PROCESSING_ON },
  })
  expect(res.ok()).toBeTruthy()
}

async function restoreProcessingViaAPI(browser: Browser) {
  const ctx = bossState
    ? await browser.newContext({ storageState: bossState })
    : await browser.newContext()
  const page = await ctx.newPage()
  try {
    if (!bossState) {
      await login(page, BOSS.username, BOSS.password)
    } else {
      // storageState 会话有效;点一下上传页确认
      await page.goto('/')
      await expect(page.getByTestId('dropzone')).toBeVisible()
    }
    await putProcessingDefault(page)
    processingDirty = false
  } finally {
    await ctx.close()
  }
}

async function uploadPNG(page: Page, name: string, buf: Buffer) {
  const chooser = page.waitForEvent('filechooser')
  await page.getByTestId('dropzone').click()
  await (await chooser).setFiles({ name, mimeType: 'image/png', buffer: buf })
}

test.afterEach(async ({ browser }) => {
  if (!processingDirty) return
  await restoreProcessingViaAPI(browser)
})

test('文字水印改变哈希:同字节先秒传、开水印后非秒传', async ({ page, browser }) => {
  // UploadCard 秒传态 DOM:badge 文案「秒传」+ sub「已存在相同文件，直接返回链接」
  // 普通成功态 badge「已完成」(status=success)。

  // 1) 注册 d2proc → 传独特 PNG A
  await page.goto('/')
  await expect(page).toHaveURL(/\/login$/)
  await page.getByRole('button', { name: '注册' }).click()
  await page.getByLabel('用户名').fill(D2PROC.username)
  await page.getByLabel('邮箱').fill(D2PROC.email)
  await page.getByLabel('密码').fill(D2PROC.password)
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  await uploadPNG(page, 'd2proc-a.png', PNG_A)
  await expect(page.getByText('已完成', { exact: true })).toHaveCount(1)
  await expect(page.getByText('秒传', { exact: true })).toHaveCount(0)

  // 2) 再传同字节 A → 秒传
  await uploadPNG(page, 'd2proc-a-again.png', PNG_A)
  await expect(page.getByText('秒传', { exact: true })).toHaveCount(1)
  await expect(page.getByText('已存在相同文件，直接返回链接')).toBeVisible()

  d2procState = await page.context().storageState()

  // 3) admin API 开文字水印(dirty 先置位);本 suite 唯一一次 boss login,缓存 storageState
  const adminCtx = await browser.newContext()
  try {
    const adminPage = await adminCtx.newPage()
    await login(adminPage, BOSS.username, BOSS.password)
    bossState = await adminCtx.storageState()
    processingDirty = true
    await putProcessingOn(adminPage)

    // 4) d2proc 第三次传同字节 A → 非秒传(烧录后哈希不同)
    // goto 重载页面,队列清空;本卡应为「已完成」而非「秒传」
    await page.goto('/')
    await expect(page.getByTestId('dropzone')).toBeVisible()
    await uploadPNG(page, 'd2proc-a-wm.png', PNG_A)
    await expect(page.getByText('已完成', { exact: true })).toBeVisible({ timeout: 30_000 })
    await expect(page.getByText('秒传', { exact: true })).toHaveCount(0)
    await expect(page.getByText('已存在相同文件，直接返回链接')).toHaveCount(0)

    // 就地还原
    await putProcessingDefault(adminPage)
    processingDirty = false
  } finally {
    await adminCtx.close()
  }
})

test('用户水印全流:上传水印图→启用偏好→上传 B→移除', async ({ browser }) => {
  expect(d2procState).toBeTruthy()
  const ctx = await browser.newContext({
    storageState: d2procState!,
    permissions: ['clipboard-read', 'clipboard-write'],
  })
  const page = await ctx.newPage()
  try {
    await page.goto('/settings/preferences')
    await expect(page.getByText('水印图', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: /上传水印图/ })).toBeVisible()

    const chooser = page.waitForEvent('filechooser')
    await page.getByRole('button', { name: /上传水印图/ }).click()
    await (await chooser).setFiles({ name: 'd2proc-wm.png', mimeType: 'image/png', buffer: PNG_WM })
    // toast 与「水印图」行内「已上传」同页并存,须 exact 避免 strict 撞 field 文本
    await expect(page.getByText('水印图已上传', { exact: true })).toBeVisible()
    await expect(page.getByText('已上传', { exact: true })).toBeVisible()

    const en = page.getByRole('switch', { name: '启用图片水印' })
    if ((await en.getAttribute('aria-checked')) !== 'true') {
      await en.click()
    }
    await page.getByRole('button', { name: '保存偏好' }).click()
    await expect(page.getByText('偏好已保存', { exact: true })).toBeVisible()

    await page.goto('/')
    await expect(page.getByTestId('dropzone')).toBeVisible()
    await uploadPNG(page, 'd2proc-b.png', PNG_B)
    // 用户水印烧录后新文件,badge 为「已完成」
    await expect(page.getByText('已完成', { exact: true }).last()).toBeVisible({ timeout: 30_000 })

    // 偏好 Tab 移除水印图(InlineConfirm 两击)
    await page.goto('/settings/preferences')
    await expect(page.getByText('已上传', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: '移除' }).click()
    await page.getByRole('button', { name: '确认移除？' }).click()
    await expect(page.getByRole('button', { name: /上传水印图/ })).toBeVisible()
  } finally {
    await ctx.close()
  }
})

test('admin 图片处理区块:UI 填 text+开启+保存再还原', async ({ browser }) => {
  expect(bossState).toBeTruthy()
  const ctx = await browser.newContext({ storageState: bossState! })
  const page = await ctx.newPage()
  try {
    await page.goto('/admin/settings')
    // 设置页已 tab 化,默认停在「基本」,先切到目标区块
    await page.getByRole('button', { name: '图片处理' }).click()
    await expect(page.getByRole('heading', { name: '图片处理' })).toBeVisible()

    const tw = page.getByRole('switch', { name: '启用文字水印' })
    if ((await tw.getAttribute('aria-checked')) !== 'true') {
      await tw.click()
    }
    // getByLabel('文字水印') 会撞 switch/slider 的 aria-label 子串,改用 textbox 角色
    await page.getByRole('textbox', { name: '文字水印' }).fill('imgli-e2e-ui')
    processingDirty = true
    await page.getByRole('button', { name: '保存设置' }).click()
    await expect(page.getByText('已保存', { exact: true })).toBeVisible()

    // 关闭还原
    if ((await tw.getAttribute('aria-checked')) === 'true') {
      await tw.click()
    }
    await page.getByRole('textbox', { name: '文字水印' }).fill('')
    await page.getByRole('button', { name: '保存设置' }).click()
    await expect(page.getByText('已保存', { exact: true })).toBeVisible()
    processingDirty = false
  } finally {
    await ctx.close()
  }
})
