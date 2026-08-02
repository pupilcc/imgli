import { expect, test } from '@playwright/test'

test('管理后台:首用户即管理员可达控制台;二号用户无入口且被重定向', async ({ page }) => {
  // 注册首个用户(自动成为管理员)；guest 关闭时 / 不强制跳登录
  await page.goto('/login')
  await page.getByRole('button', { name: '注册' }).click()
  await page.getByLabel('用户名').fill('boss')
  await page.getByLabel('邮箱').fill('boss@img.li')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  // 头像菜单入口 → 控制台
  await page.locator('header button[class*=avatar]').click()
  await page.getByRole('link', { name: '管理后台' }).click()
  await expect(page).toHaveURL(/\/admin$/)
  await expect(page.getByRole('heading', { name: '仪表盘' })).toBeVisible()
  await expect(page.getByText('用户数')).toBeVisible()
  await expect(page.getByRole('link', { name: /用户组/ })).toBeVisible()

  // 系统设置已在④d交付为真实页面
  await page.getByRole('link', { name: /系统设置/ }).click()
  await expect(page.getByRole('button', { name: '保存设置' })).toBeVisible()

  // 返回前台
  await page.getByRole('link', { name: '← 返回前台' }).click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  // 退出登录
  await page.locator('header button[class*=avatar]').click()
  await page.getByRole('button', { name: '退出登录' }).click()
  await expect(page).toHaveURL(/\/login$/)

  // 二号用户:非 admin
  await page.getByRole('button', { name: '注册' }).click()
  await page.getByLabel('用户名').fill('pleb')
  await page.getByLabel('邮箱').fill('pleb@img.li')
  await page.getByLabel('密码').fill('plebpass-777')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  // 菜单无「管理后台」
  await page.locator('header button[class*=avatar]').click()
  await expect(page.getByRole('button', { name: '退出登录' })).toBeVisible()
  await expect(page.getByRole('link', { name: '管理后台' })).toHaveCount(0)

  // 直达 /admin 被重定向回前台
  await page.goto('/admin')
  await expect(page.getByTestId('dropzone')).toBeVisible()
})

// 与 main.spec 的 1x1 红色 PNG 内容不同(像素值不同 → hash 不同),避免撞上秒传去重——
// 若用同一份字节,本用例先跑会让 main.spec 后续上传命中秒传,把断言的「已完成」变成「秒传」。
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR42mPgEpEDAABoAD1q9XBbAAAAAElFTkSuQmCC',
  'base64',
)

test('管理后台:用户管理与图片管理链路', async ({ page }) => {
  // boss 登录(首个 test 已注册)
  await page.goto('/login')
  await page.getByLabel('账号').fill('boss')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  // 上传一张图供图片管理操作
  const chooser = page.waitForEvent('filechooser')
  await page.getByTestId('dropzone').click()
  await (await chooser).setFiles({ name: 'admin-e2e.png', mimeType: 'image/png', buffer: PNG })
  await expect(page.getByText('已完成', { exact: true })).toBeVisible()

  // 图片管理:可见 → 加白(两击)→ WL 徽章
  await page.goto('/admin/images')
  await expect(page.getByText('admin-e2e.png')).toBeVisible()
  const card = page.locator('main [class*=card]').first()
  await card.hover()
  await card.getByTitle('加白').click()
  await card.getByTitle('确认加白').click()
  await expect(page.getByText('WL')).toBeVisible()

  // 删除(两击)→ 空态或不再可见
  const card2 = page.locator('main [class*=card]').first()
  await card2.hover()
  await card2.getByTitle('移入回收站').click()
  await card2.getByTitle('确认移入回收站').click()
  await expect(page.getByText('admin-e2e.png')).toHaveCount(0)

  // 用户管理:搜 pleb → 封禁 → 已封禁 → 解封
  await page.goto('/admin/users')
  await page.getByPlaceholder('搜索用户名 / 邮箱…').fill('pleb')
  // 300ms 防抖后 URL 才带上 q,列表才真正按关键字重新拉取;否则 boss/pleb 两行仍都在,下面按钮定位会撞车
  await expect(page).toHaveURL(/[?&]q=pleb/)
  await expect(page.getByText('boss@img.li')).toHaveCount(0)
  await expect(page.getByText('pleb@img.li')).toBeVisible()
  await page.getByRole('button', { name: '封禁' }).click()
  await page.getByRole('button', { name: '确认封禁？' }).click()
  // 状态 tag 用 class 定位——「已封禁/正常」文本与筛选 select 的 option 撞车,strict 模式不可用 getByText
  await expect(page.locator('main [class*=stErr]')).toBeVisible()
  await page.getByRole('button', { name: '解封' }).click()
  await expect(page.locator('main [class*=stOk]')).toBeVisible()

  // 重置密码 Modal
  await page.getByRole('button', { name: '重置密码' }).click()
  await page.getByRole('button', { name: '确认重置' }).click()
  await expect(page.getByText(/仅显示一次/)).toBeVisible()
  await page.getByRole('button', { name: '关闭' }).click()
})

test('管理后台:用户组新建删除 + 存储策略测试连接', async ({ page }) => {
  // boss 登录(首个 test 已注册)
  await page.goto('/login')
  await page.getByLabel('账号').fill('boss')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  // 用户组:新建 → 保存 → 列表可见 → 选中 → 删除确认 → 不再可见
  await page.goto('/admin/groups')
  await page.getByRole('button', { name: /新建组/ }).click()
  await page.getByLabel('组名').fill('E2E组')
  await page.getByRole('button', { name: '保存' }).click()
  await expect(page.getByText('E2E组')).toBeVisible()
  await page.getByRole('button', { name: /E2E组/ }).click()
  await page.getByRole('button', { name: '删除' }).click()
  await page.getByRole('button', { name: '确认删除？' }).click()
  await expect(page.getByText('E2E组')).toHaveCount(0)

  // 存储策略:选中种子「本地存储」→ 路径/驱动回显 → 测试连接成功
  await page.goto('/admin/policies')
  await page.getByRole('button', { name: /本地存储/ }).click()
  await expect(page.getByLabel('存储路径')).toHaveValue('uploads')
  // Caps 摘要也含「本地磁盘：…」，须 exact，避免 strict mode 双命中
  await expect(page.getByText('本地磁盘', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '测试连接' }).click()
  await expect(page.getByText(/已连接 ·/)).toBeVisible()
})

test('管理后台:保存设置落审计 + 日志可见可展开', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('账号').fill('boss')
  await page.getByLabel('密码').fill('bosspass-777')
  await page.getByTestId('auth-submit').click()
  await expect(page.getByTestId('dropzone')).toBeVisible()

  // 保存设置:改站点名 → 保存(落 settings_update 审计)
  await page.goto('/admin/settings')
  const name = page.getByLabel('站点名称')
  await name.fill('imgli-e2e')
  await page.getByRole('button', { name: '保存设置' }).click()
  await expect(page.getByText('已保存')).toBeVisible()

  // 操作日志:出现「更新系统设置」→ 点行展开看到 detail 原文(含 fields)
  // 注意:「更新系统设置」同时出现在页头「操作筛选」下拉的 <option> 里(DOM 序早于表格行,
  // 且 <option> 不算 visible),故不能用 getByText(...).first()——需用 role=button 精确定位日志行本身。
  await page.goto('/admin/logs')
  const row = page.getByRole('button', { name: /更新系统设置/ }).first()
  await expect(row).toBeVisible()
  await row.click()
  await expect(page.getByText(/site_name/).first()).toBeVisible()
})
