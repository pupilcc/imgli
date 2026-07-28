import { expect, test } from '@playwright/test'

// PWA:manifest/sw 服务 + SW 注册激活 + 离线应用壳仍加载。
// e2e webServer 服务生产构建(import.meta.env.PROD=true),registerSW 会注册 /sw.js。
test('PWA:manifest/sw 服务 + SW 注册 + 离线应用壳', async ({ page, context }) => {
  await page.goto('/')

  // manifest link 存在且资源可服务
  await expect(page.locator('link[rel="manifest"]')).toHaveAttribute('href', '/manifest.webmanifest')
  expect((await page.request.get('/manifest.webmanifest')).status()).toBe(200)
  expect((await page.request.get('/sw.js')).status()).toBe(200)
  expect((await page.request.get('/pwa/icon-192.png')).status()).toBe(200)

  // SW 注册并激活(skipWaiting+clients.claim → controller 接管)
  await page.waitForFunction(async () => {
    if (!('serviceWorker' in navigator)) return false
    const r = await navigator.serviceWorker.ready
    return !!r.active
  }, null, { timeout: 15_000 })

  // 触发一次受控加载,确保壳 '/' 与静态资源进缓存
  await page.waitForFunction(() => navigator.serviceWorker.controller != null, null, { timeout: 15_000 })
  await page.reload()
  await expect(page.locator('#root')).toBeVisible()

  // 离线:reload 应用壳仍加载(SW 导航回落缓存壳 + 静态 cache-first),非浏览器离线错误页
  await context.setOffline(true)
  await page.reload()
  await expect(page.locator('#root')).toBeVisible()
  // 根有实际渲染内容(壳起来了)
  const hasContent = await page.evaluate(() => (document.getElementById('root')?.childElementCount ?? 0) > 0)
  expect(hasContent).toBe(true)

  await context.setOffline(false)
})
