import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { SettingsPage } from './SettingsPage'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

const SETTINGS = {
  site_name: '白栗',
  registration_mode: 'open',
  guest_upload_enabled: false,
  plaza_enabled: false,
  moderation: {
    enabled: true,
    provider: 'webhook',
    endpoint: 'https://mod.example',
    api_key: '****cdef',
    access_key_id: '',
    access_key_secret: '',
    region: '',
    threshold: 0.8,
    action: 'pending',
    ocr_keywords: {
      enabled: false,
      endpoint: '',
      api_key: '',
      keywords: [],
      on_hit: 'review',
    },
  },
  smtp: { host: '', port: 587, username: '', password: '', from: '', encryption: 'starttls' },
  hotlink: { enabled: false, allowed_domains: [], allow_empty_referer: true },
  processing: {
    text_watermark: { enabled: false, text: '', position: 'br', opacity: 0.35, size_ratio: 0.05 },
    max_edge: 0,
    strip_exif: true,
  },
}

let putBody: Record<string, unknown> | null = null
function mockBackend() {
  putBody = null
  vi.stubGlobal('fetch', vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
    const u = String(url)
    if (u.includes('/admin/settings/smtp/test') && init?.method === 'POST') {
      return Promise.resolve(jsonRes(env({})))
    }
    if (u.includes('/admin/settings/moderation/test') && init?.method === 'POST') {
      return Promise.resolve(jsonRes(env({ score: 0.42 })))
    }
    if (u.includes('/admin/settings') && init?.method === 'PUT') {
      putBody = JSON.parse(String(init.body))
      return Promise.resolve(jsonRes(env({ ...SETTINGS, ...(putBody as object) })))
    }
    if (u.includes('/admin/settings')) return Promise.resolve(jsonRes(env(SETTINGS)))
    return Promise.resolve(jsonRes(env(null)))
  }))
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/settings']}>
        <SettingsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

/** 点未激活的分区 Tab（aria-pressed=false），避免与区内同名按钮冲突 */
async function openTab(name: string) {
  await userEvent.click(screen.getByRole('button', { name, pressed: false }))
}

afterEach(() => vi.unstubAllGlobals())

it('加载回显:站点名、打码 key、阈值', async () => {
  mockBackend()
  renderPage()
  expect(await screen.findByLabelText('站点名称')).toHaveValue('白栗')
  await openTab('机器审核')
  expect(screen.getByLabelText('API Key')).toHaveValue('****cdef')
  expect(screen.getByText('0.80')).toBeInTheDocument()
})

it('改站点名保存:全量 PUT 四键,api_key 原样打码回传', async () => {
  mockBackend()
  renderPage()
  const name = await screen.findByLabelText('站点名称')
  await userEvent.clear(name)
  await userEvent.type(name, '新站点')
  await userEvent.click(screen.getByRole('button', { name: '保存设置' }))
  await waitFor(() => expect(putBody).toBeTruthy())
  expect(putBody).toMatchObject({
    site_name: '新站点',
    registration_mode: 'open',
    guest_upload_enabled: false,
    moderation: {
      enabled: true,
      provider: 'webhook',
      endpoint: 'https://mod.example',
      api_key: '****cdef',
      access_key_id: '',
      access_key_secret: '',
      region: '',
      threshold: 0.8,
      action: 'pending',
      ocr_keywords: {
        enabled: false,
        endpoint: '',
        api_key: '',
        keywords: [],
        on_hit: 'review',
      },
    },
  })
})

it('OCR 词表:开启+endpoint+多行词表保存进 ocr_keywords', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('OCR 词表审核')
  expect(screen.getByRole('heading', { name: 'OCR 词表审核' })).toBeInTheDocument()
  await userEvent.click(screen.getByLabelText('启用 OCR 词表'))
  await userEvent.type(screen.getByLabelText('OCR 服务地址'), 'http://ocr.example:3199/')
  const kw = screen.getByLabelText('敏感词表')
  await userEvent.clear(kw)
  await userEvent.type(kw, 'foo\n# comment\nbar\nfoo')
  expect(screen.getByText('2 词')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: '保存设置' }))
  await waitFor(() => expect(putBody).toBeTruthy())
  const ocr = (putBody as {
    moderation: {
      ocr_keywords: {
        enabled: boolean
        endpoint: string
        keywords: string[]
        on_hit: string
      }
    }
  }).moderation.ocr_keywords
  expect(ocr.enabled).toBe(true)
  expect(ocr.endpoint).toBe('http://ocr.example:3199/')
  expect(ocr.keywords).toEqual(['foo', 'bar'])
  expect(ocr.on_hit).toBe('review')
})

it('OCR 词表工具条:导入合并/替换/导出按钮存在', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('OCR 词表审核')
  expect(screen.getByRole('button', { name: '导入合并' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '导入替换' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '导出 .txt' })).toBeDisabled()
})

it('机审 provider 切 openai → API Key 显示、Webhook 地址隐藏;切 aliyun → AccessKey 显示、API Key 隐藏', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('机器审核')
  expect(screen.getByLabelText('Webhook 地址')).toBeInTheDocument()
  expect(screen.getByLabelText('API Key')).toBeInTheDocument()

  await userEvent.click(screen.getByRole('button', { name: 'OpenAI' }))
  expect(screen.queryByLabelText('Webhook 地址')).not.toBeInTheDocument()
  expect(screen.getByLabelText('API Key')).toBeInTheDocument()
  expect(screen.queryByLabelText('AccessKey ID')).not.toBeInTheDocument()

  await userEvent.click(screen.getByRole('button', { name: '阿里云' }))
  expect(screen.getByLabelText('AccessKey ID')).toBeInTheDocument()
  expect(screen.getByLabelText('AccessKey Secret')).toBeInTheDocument()
  expect(screen.getByLabelText('Region')).toBeInTheDocument()
  expect(screen.queryByLabelText('API Key')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('Webhook 地址')).not.toBeInTheDocument()
})

it('保存 provider aliyun + AKID/Region → PUT body 正确且 threshold/action 仍在', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('机器审核')
  await userEvent.click(screen.getByRole('button', { name: '阿里云' }))
  await userEvent.type(screen.getByLabelText('AccessKey ID'), 'LTAI5tExample')
  await userEvent.type(screen.getByLabelText('Region'), 'cn-shanghai')
  await userEvent.click(screen.getByRole('button', { name: '保存设置' }))
  await waitFor(() => expect(putBody).toBeTruthy())
  const mod = (putBody as {
    moderation: {
      provider: string
      access_key_id: string
      region: string
      threshold: number
      action: string
      enabled: boolean
    }
  }).moderation
  expect(mod.provider).toBe('aliyun')
  expect(mod.access_key_id).toBe('LTAI5tExample')
  expect(mod.region).toBe('cn-shanghai')
  expect(mod.threshold).toBe(0.8)
  expect(mod.action).toBe('pending')
  expect(mod.enabled).toBe(true)
})

it('切 provider(webhook→openai)不携带旧 provider 掩码 key,发空串(codex 终审 F1)', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('机器审核')
  await userEvent.click(screen.getByRole('button', { name: 'OpenAI' }))
  await userEvent.click(screen.getByRole('button', { name: '保存设置' }))
  await waitFor(() => expect(putBody).toBeTruthy())
  const mod = (putBody as { moderation: { provider: string; api_key: string } }).moderation
  expect(mod.provider).toBe('openai')
  // 初始 webhook 的 ****cdef 掩码 key 不得随 provider 切换携带(否则后端「改指向即失效」拒 400)
  expect(mod.api_key).toBe('')
})

it('切注册模式为关闭并保存', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await userEvent.click(screen.getByRole('button', { name: '关闭注册' }))
  await userEvent.click(screen.getByRole('button', { name: '保存设置' }))
  await waitFor(() => expect((putBody as { registration_mode: string }).registration_mode).toBe('closed'))
})

it('切注册模式为邀请并保存', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await userEvent.click(screen.getByRole('button', { name: '邀请注册' }))
  await userEvent.click(screen.getByRole('button', { name: '保存设置' }))
  await waitFor(() => expect((putBody as { registration_mode: string }).registration_mode).toBe('invite'))
})

it('开启允许游客上传并保存', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await userEvent.click(screen.getByRole('switch', { name: '允许游客上传' }))
  await userEvent.click(screen.getByRole('button', { name: '保存设置' }))
  await waitFor(() => expect((putBody as { guest_upload_enabled: boolean }).guest_upload_enabled).toBe(true))
})

it('API Key 聚焦时全选', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('机器审核')
  const apiKeyInput = screen.getByLabelText('API Key') as HTMLInputElement
  const maskedValue = apiKeyInput.value
  expect(maskedValue).toBe('****cdef')
  apiKeyInput.focus()
  expect(apiKeyInput.selectionStart).toBe(0)
  expect(apiKeyInput.selectionEnd).toBe(maskedValue.length)
})

it('填写 SMTP 并保存:PUT 带 smtp 五键', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('邮件 SMTP')
  await userEvent.type(screen.getByLabelText('SMTP 服务器'), 'smtp.example')
  await userEvent.type(screen.getByLabelText('发件人'), 'no-reply@img.li')
  await userEvent.click(screen.getByRole('button', { name: 'SSL' }))
  await userEvent.click(screen.getByRole('button', { name: '保存设置' }))
  await waitFor(() => expect(putBody).toBeTruthy())
  expect((putBody as { smtp: { host: string; encryption: string; from: string } }).smtp).toMatchObject({
    host: 'smtp.example', from: 'no-reply@img.li', encryption: 'ssl',
  })
})

it('发送测试邮件:调 test 端点并显示行内结果', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('邮件 SMTP')
  await userEvent.type(screen.getByLabelText('测试收件人'), 'me@img.li')
  await userEvent.click(screen.getByRole('button', { name: '发送测试邮件' }))
  expect(await screen.findByText('已发送,请查收')).toBeInTheDocument()
})

it('防盗链区块:渲染三控件', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('防盗链')
  expect(screen.getByRole('heading', { name: '防盗链' })).toBeInTheDocument()
  expect(screen.getByRole('switch', { name: '启用防盗链' })).toBeInTheDocument()
  expect(screen.getByLabelText('允许的来源域名')).toBeInTheDocument()
  expect(screen.getByRole('switch', { name: '允许空 Referer' })).toBeInTheDocument()
})

it('防盗链:域名换行拆分 + 启用后全量 PUT', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('防盗链')
  await userEvent.click(screen.getByRole('switch', { name: '启用防盗链' }))
  const domains = screen.getByLabelText('允许的来源域名')
  await userEvent.clear(domains)
  await userEvent.type(domains, 'a.example\n*.b.example')
  await userEvent.click(screen.getByRole('button', { name: '保存设置' }))
  await waitFor(() => expect(putBody).toBeTruthy())
  expect((putBody as { hotlink: { enabled: boolean; allowed_domains: string[] } }).hotlink).toMatchObject({
    enabled: true,
    allowed_domains: ['a.example', '*.b.example'],
  })
})

it('图片处理区块:渲染文字水印与最长边', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('图片处理')
  expect(screen.getByRole('heading', { name: '图片处理' })).toBeInTheDocument()
  expect(screen.getByRole('switch', { name: '启用文字水印' })).toBeInTheDocument()
  expect(screen.getByLabelText('文字水印')).toBeInTheDocument()
  expect(screen.getByLabelText('水印位置')).toBeInTheDocument()
  expect(screen.getByLabelText('字号比例')).toBeInTheDocument()
  expect(screen.getByLabelText('最长边')).toBeInTheDocument()
})

it('图片处理:填 text+开启+保存 → PUT processing 正确且 max_edge 数值化', async () => {
  mockBackend()
  renderPage()
  await screen.findByLabelText('站点名称')
  await openTab('图片处理')
  await userEvent.click(screen.getByRole('switch', { name: '启用文字水印' }))
  await userEvent.type(screen.getByLabelText('文字水印'), '白栗图床')
  const maxEdge = screen.getByLabelText('最长边')
  await userEvent.clear(maxEdge)
  await userEvent.type(maxEdge, '2048')
  await userEvent.click(screen.getByRole('button', { name: '保存设置' }))
  await waitFor(() => expect(putBody).toBeTruthy())
  const processing = (putBody as {
    processing: {
      text_watermark: { enabled: boolean; text: string }
      max_edge: number
    }
  }).processing
  expect(processing.text_watermark.enabled).toBe(true)
  expect(processing.text_watermark.text).toBe('白栗图床')
  expect(processing.max_edge).toBe(2048)
  expect(typeof processing.max_edge).toBe('number')
})

it('Tab 切换:只展示当前分区,表单状态跨 Tab 保留', async () => {
  mockBackend()
  renderPage()
  const name = await screen.findByLabelText('站点名称')
  await userEvent.clear(name)
  await userEvent.type(name, '跨 Tab')
  expect(screen.queryByLabelText('SMTP 服务器')).not.toBeInTheDocument()
  await openTab('邮件 SMTP')
  expect(screen.getByLabelText('SMTP 服务器')).toBeInTheDocument()
  expect(screen.queryByLabelText('站点名称')).not.toBeInTheDocument()
  await openTab('基本')
  expect(screen.getByLabelText('站点名称')).toHaveValue('跨 Tab')
})
