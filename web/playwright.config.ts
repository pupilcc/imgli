import { defineConfig } from '@playwright/test'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const dataDir = mkdtempSync(join(tmpdir(), 'imgli-e2e-'))

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  retries: 0,
  workers: 1,
  use: {
    baseURL: 'http://localhost:8697',
    // 锁浏览器 locale zh-CN,使 i18n detectLang 返 zh、全站默认中文——现有 spec 断言中文
    // (Playwright 默认 navigator.language=en-US 会让 detectLang 返 en 渲染英文致断言崩)。
    // lang.spec 显式覆盖此项测切换。镜像 vitest.setup 的 imgli-lang=zh。
    locale: 'zh-CN',
    trace: 'retain-on-failure',
    // headless chromium 下 Async Clipboard API 默认无权限，writeText 会静默 reject
    // 导致 copyText 落入 catch 分支弹「复制失败」而非「已复制」，与断言链路预期不符。
    permissions: ['clipboard-read', 'clipboard-write'],
  },
  webServer: {
    command: '../imgli serve',
    url: 'http://localhost:8697/healthz',
    env: {
      IMGLI_LISTEN: ':8697',
      IMGLI_BASE_URL: 'http://localhost:8697',
      IMGLI_DATA_DIR: dataDir,
      // e2e 全套件跨 spec 累积注册/登录远超生产 auth 桶(20/min);整体放宽避免
      // 跨 spec 429 耦合(否则须在 spec 里 sleep 等窗口过期,拖慢且脆弱)。生产不设此变量。
      IMGLI_RATE_LIMIT_MULT: '100',
    },
    reuseExistingServer: false,
    timeout: 15_000,
  },
})
