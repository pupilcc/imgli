import { expect, test } from '@playwright/test'

// 与 admin/main/guest 的 1x1 PNG 字节均不同,避免秒传串扰
const AVATAR_PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
  'base64',
)
// 注销流上传用图(再换一组像素)
const DEL_PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR42mP8z/C/HgAGgwJ/lK3Q6wAAAABJRU5ErkJggg==',
  'base64',
)

const PREF_USER = { username: 'c3pref', email: 'c3pref@img.li', password: 'c3prefpass-777' }
const DEL_USER = { username: 'c3del', email: 'c3del@img.li', password: 'c3delpass-777' }

test.describe.configure({ mode: 'serial' })

test.describe('C-③ 设置：偏好 / 头像 / 注销', () => {
  test('偏好在上传页生效', async ({ page }) => {
    await page.goto('/login')
    await page.getByRole('button', { name: '注册' }).click()
    await page.getByLabel('用户名').fill(PREF_USER.username)
    await page.getByLabel('邮箱').fill(PREF_USER.email)
    await page.getByLabel('密码').fill(PREF_USER.password)
    await page.getByTestId('auth-submit').click()
    await expect(page.getByTestId('dropzone')).toBeVisible()

    await page.goto('/settings/preferences')
    await expect(page.getByText('默认可见性')).toBeVisible()
    await page.getByRole('button', { name: '私密' }).click()
    await page.getByRole('button', { name: '保存偏好' }).click()
    await expect(page.getByText('偏好已保存')).toBeVisible()

    await page.goto('/')
    await expect(page.getByTestId('dropzone')).toBeVisible()
    // 选项 summary 常驻显示「相册 · 可见性」
    await expect(page.getByText(/· 私密/)).toBeVisible()
  })

  test('头像上传与移除', async ({ page }) => {
    await page.goto('/login')
    await page.getByLabel('账号').fill(PREF_USER.username)
    await page.getByLabel('密码').fill(PREF_USER.password)
    await page.getByTestId('auth-submit').click()
    await expect(page.getByTestId('dropzone')).toBeVisible()

    await page.goto('/settings')
    await expect(page.getByRole('button', { name: '上传头像' })).toBeVisible()

    const chooser = page.waitForEvent('filechooser')
    await page.getByRole('button', { name: '上传头像' }).click()
    await (await chooser).setFiles({ name: 'c3-avatar.png', mimeType: 'image/png', buffer: AVATAR_PNG })
    await expect(page.getByText('头像已更新')).toBeVisible()

    const avatarBtn = page.locator('header button[class*=avatar]')
    await expect(avatarBtn.locator('img')).toBeVisible()

    await page.getByRole('button', { name: '移除头像' }).click()
    await page.getByRole('button', { name: '确认移除？' }).click()
    await expect(avatarBtn.locator('img')).toHaveCount(0)
    // 回退首字母(昵称空则用户名首字)
    await expect(avatarBtn).toHaveText(PREF_USER.username.slice(0, 1))
  })

  test('注销全流', async ({ page, request }) => {
    await page.goto('/login')
    await page.getByRole('button', { name: '注册' }).click()
    await page.getByLabel('用户名').fill(DEL_USER.username)
    await page.getByLabel('邮箱').fill(DEL_USER.email)
    await page.getByLabel('密码').fill(DEL_USER.password)
    await page.getByTestId('auth-submit').click()
    await expect(page.getByTestId('dropzone')).toBeVisible()

    const chooser = page.waitForEvent('filechooser')
    await page.getByTestId('dropzone').click()
    await (await chooser).setFiles({ name: 'c3-del-shot.png', mimeType: 'image/png', buffer: DEL_PNG })
    await expect(page.getByText('已完成', { exact: true })).toBeVisible()
    const directUrl = (await page.locator('[class*=urlText]').first().textContent())?.trim()
    expect(directUrl).toBeTruthy()
    expect(directUrl!).toMatch(/^https?:\/\//)

    await page.goto('/settings')
    await expect(page.getByText(/危险区/)).toBeVisible()
    await page.getByLabel('输入当前密码以确认').fill(DEL_USER.password)
    await page.getByRole('button', { name: '永久注销账号' }).click()
    await page.getByRole('button', { name: '再次点击确认注销' }).click()
    await expect(page).toHaveURL(/\/login$/)

    const res = await request.get(directUrl!)
    expect(res.status()).toBe(404)

    // 用户名已释放,可重新注册
    await page.getByRole('button', { name: '注册' }).click()
    await page.getByLabel('用户名').fill(DEL_USER.username)
    await page.getByLabel('邮箱').fill(DEL_USER.email)
    await page.getByLabel('密码').fill(DEL_USER.password)
    await page.getByTestId('auth-submit').click()
    await expect(page.getByTestId('dropzone')).toBeVisible()
  })
})
