import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { PoliciesPage } from './PoliciesPage'

function jsonRes(body: unknown, status = 200): Response {
  return { ok: status < 400, status, json: () => Promise.resolve(body) } as unknown as Response
}
const env = (data: unknown) => ({ status: true, message: 'ok', data })

const policies = [
  { id: 1, name: '本地默认', driver: 'local', config: '{"root":"/data/uploads"}', cdn_domain: 'https://img.li', path_template: '{Y}/{m}/{d}/{uniqid}.{ext}', enabled: true, created_at: '', file_count: 12, used_bytes: 1024 },
  {
    id: 2,
    name: 'S3生产',
    driver: 's3',
    config: JSON.stringify({
      endpoint: 's3.us-east-1.amazonaws.com',
      region: 'us-east-1',
      bucket: 'my-bucket',
      access_key_id: 'AKIATEST',
      secret_access_key: '****abcd',
      path_style: 'false',
      prefix: 'imgli/',
    }),
    cdn_domain: 'https://cdn.example',
    path_template: '{Y}/{m}/{d}/{uniqid}.{ext}',
    enabled: true,
    created_at: '',
    file_count: 3,
    used_bytes: 2048,
  },
  {
    id: 3,
    name: 'WebDAV生产',
    driver: 'webdav',
    config: JSON.stringify({
      endpoint: 'https://dav.example.com/imgli',
      username: 'davuser',
      password: '****wxyz',
    }),
    cdn_domain: '',
    path_template: '{Y}/{m}/{d}/{uniqid}.{ext}',
    enabled: true,
    created_at: '',
    file_count: 1,
    used_bytes: 512,
  },
]

let created: unknown = null
let patched: { id: string; body: unknown } | null = null
let tested: string | null = null
function mockBackend() {
  created = null
  patched = null
  tested = null
  vi.stubGlobal('fetch', vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
    const u = String(url)
    const m = init?.method
    if (u.includes('/test')) {
      tested = u
      return Promise.resolve(jsonRes(env({ ok: true, latency_ms: 3 })))
    }
    if (u.match(/\/admin\/policies\/\d+/) && m === 'PATCH') {
      patched = { id: u.split('/').pop()!, body: JSON.parse(String(init!.body)) }
      return Promise.resolve(jsonRes(env({ ...policies[0], ...(patched.body as object) })))
    }
    if (u.includes('/admin/policies') && m === 'POST') {
      created = JSON.parse(String(init!.body))
      return Promise.resolve(jsonRes(env({ id: 9 })))
    }
    if (u.includes('/admin/policies')) return Promise.resolve(jsonRes(env({ items: policies })))
    return Promise.resolve(jsonRes(env(null)))
  }))
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/admin/policies']}>
        <PoliciesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

afterEach(() => vi.unstubAllGlobals())

it('列表 + 载入表单:root 从 config 解析', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByText('本地默认'))
  expect(screen.getByLabelText('存储路径')).toHaveValue('/data/uploads')
  expect(screen.getByText('本地磁盘')).toBeInTheDocument()
})

it('编辑:全量提交,config 序列化 root', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByText('本地默认'))
  const root = screen.getByLabelText('存储路径')
  await userEvent.clear(root)
  await userEvent.type(root, '/srv/img')
  await userEvent.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(patched?.id).toBe('1'))
  expect((patched!.body as { config: string }).config).toBe('{"root":"/srv/img"}')
})

it('测试连接:回显延迟', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByText('本地默认'))
  await userEvent.click(screen.getByRole('button', { name: '测试连接' }))
  await waitFor(() => expect(tested).toContain('/admin/policies/1/test'))
  expect(await screen.findByText(/已连接 · 3ms/)).toBeInTheDocument()
})

it('新建:driver=local + config root', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByRole('button', { name: /新建策略/ }))
  await userEvent.type(screen.getByLabelText('名称'), '备用盘')
  await userEvent.type(screen.getByLabelText('存储路径'), '/mnt/x')
  await userEvent.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(created).toMatchObject({ name: '备用盘', driver: 'local', config: '{"root":"/mnt/x"}' }))
})

it('新建态选 S3 驱动:s3 字段出现,存储路径隐藏', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByRole('button', { name: /新建策略/ }))
  expect(screen.getByLabelText('存储路径')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'S3' }))
  expect(screen.queryByLabelText('存储路径')).not.toBeInTheDocument()
  expect(screen.getByLabelText('Endpoint')).toBeInTheDocument()
  expect(screen.getByLabelText('Region')).toBeInTheDocument()
  expect(screen.getByLabelText('Bucket')).toBeInTheDocument()
  expect(screen.getByLabelText('AccessKey ID')).toBeInTheDocument()
  expect(screen.getByLabelText('AccessKey Secret')).toBeInTheDocument()
})

it('新建 s3 策略:POST driver=s3 且 config 含 endpoint/bucket/path_style', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByRole('button', { name: /新建策略/ }))
  await userEvent.click(screen.getByRole('button', { name: 'S3' }))
  await userEvent.type(screen.getByLabelText('名称'), 'MinIO')
  await userEvent.type(screen.getByLabelText('Endpoint'), 'minio.local:9000')
  await userEvent.type(screen.getByLabelText('Region'), 'us-east-1')
  await userEvent.type(screen.getByLabelText('Bucket'), 'imgli')
  await userEvent.type(screen.getByLabelText('AccessKey ID'), 'minioadmin')
  await userEvent.type(screen.getByLabelText('AccessKey Secret'), 'minioadmin')
  await userEvent.click(screen.getByRole('button', { name: '路径风格' }))
  await userEvent.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(created).not.toBeNull())
  const body = created as { driver: string; config: string }
  expect(body.driver).toBe('s3')
  const cfg = JSON.parse(body.config) as {
    endpoint: string
    region: string
    bucket: string
    access_key_id: string
    secret_access_key: string
    path_style: string
    prefix: string
  }
  expect(cfg.endpoint).toBe('minio.local:9000')
  expect(cfg.bucket).toBe('imgli')
  expect(cfg.access_key_id).toBe('minioadmin')
  expect(cfg.secret_access_key).toBe('minioadmin')
  expect(cfg.path_style).toBe('true')
  expect(cfg.region).toBe('us-east-1')
})

it('编辑 s3:secret 显打码,聚焦全选;不改 secret 保存原样回传', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByText('S3生产'))
  const secret = screen.getByLabelText('AccessKey Secret') as HTMLInputElement
  expect(secret).toHaveValue('****abcd')
  expect(screen.getByText(/已设密钥显示为掩码/)).toBeInTheDocument()
  secret.focus()
  expect(secret.selectionStart).toBe(0)
  expect(secret.selectionEnd).toBe('****abcd'.length)
  await userEvent.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(patched?.id).toBe('2'))
  const cfg = JSON.parse((patched!.body as { config: string }).config) as { secret_access_key: string }
  expect(cfg.secret_access_key).toBe('****abcd')
})

it('新建态选 WebDAV 驱动:webdav 字段出现,s3 与 local 字段隐藏', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByRole('button', { name: /新建策略/ }))
  expect(screen.getByLabelText('存储路径')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'WebDAV' }))
  expect(screen.queryByLabelText('存储路径')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('Region')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('Bucket')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('AccessKey Secret')).not.toBeInTheDocument()
  expect(screen.getByLabelText('Endpoint')).toBeInTheDocument()
  expect(screen.getByLabelText('用户名')).toBeInTheDocument()
  expect(screen.getByLabelText('密码')).toBeInTheDocument()
})

it('新建 webdav 策略:POST driver=webdav 且 config 含 endpoint/username', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByRole('button', { name: /新建策略/ }))
  await userEvent.click(screen.getByRole('button', { name: 'WebDAV' }))
  await userEvent.type(screen.getByLabelText('名称'), 'NAS')
  await userEvent.type(screen.getByLabelText('Endpoint'), 'https://dav.example.com/imgli')
  await userEvent.type(screen.getByLabelText('用户名'), 'davuser')
  await userEvent.type(screen.getByLabelText('密码'), 'secret')
  await userEvent.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(created).not.toBeNull())
  const body = created as { driver: string; config: string }
  expect(body.driver).toBe('webdav')
  const cfg = JSON.parse(body.config) as { endpoint: string; username: string; password: string }
  expect(cfg.endpoint).toBe('https://dav.example.com/imgli')
  expect(cfg.username).toBe('davuser')
  expect(cfg.password).toBe('secret')
})

it('编辑 webdav:password 显打码,聚焦全选;不改 password 保存原样回传', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByText('WebDAV生产'))
  const password = screen.getByLabelText('密码') as HTMLInputElement
  expect(password).toHaveValue('****wxyz')
  expect(screen.getByText(/已设密钥显示为掩码/)).toBeInTheDocument()
  password.focus()
  expect(password.selectionStart).toBe(0)
  expect(password.selectionEnd).toBe('****wxyz'.length)
  await userEvent.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(patched?.id).toBe('3'))
  const cfg = JSON.parse((patched!.body as { config: string }).config) as { password: string }
  expect(cfg.password).toBe('****wxyz')
})

it('新建 s3 策略:config 含 presign_domain', async () => {
  mockBackend()
  renderPage()
  await userEvent.click(await screen.findByRole('button', { name: /新建策略/ }))
  await userEvent.click(screen.getByRole('button', { name: 'S3' }))
  await userEvent.type(screen.getByLabelText('名称'), 'RustFS')
  await userEvent.type(screen.getByLabelText('Endpoint'), '192.0.2.10:9000')
  await userEvent.type(screen.getByLabelText('Region'), 'us-east-1')
  await userEvent.type(screen.getByLabelText('Bucket'), 'imgli')
  await userEvent.type(screen.getByLabelText('AccessKey ID'), 'AK')
  await userEvent.type(screen.getByLabelText('AccessKey Secret'), 'SK')
  await userEvent.type(screen.getByLabelText('预签名直连域'), 'https://s3.img.li')
  await userEvent.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(created).not.toBeNull())
  const body = created as { config: string }
  const cfg = JSON.parse(body.config) as { presign_domain: string }
  expect(cfg.presign_domain).toBe('https://s3.img.li')
})
