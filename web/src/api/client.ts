import { t } from '../i18n'

export class ApiError extends Error {
  constructor(
    public httpStatus: number,
    public code: string,
    message: string,
    /** 秒；来自 Retry-After（若有） */
    public retryAfterSec?: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

function parseRetryAfter(res: Response): number | undefined {
  try {
    const raw = res.headers?.get?.('Retry-After')
    if (!raw) return undefined
    const n = Number(raw)
    if (Number.isFinite(n) && n > 0) return Math.ceil(n)
  } catch {
    /* test mocks may omit headers */
  }
  return undefined
}

interface Envelope<T> {
  status: boolean
  message: string
  data: T
}

let onUnauthorized: (() => void) | null = null
let onForbidden: (() => void) | null = null

/** 注册全局 401 处理（App 设为跳登录页）；登录失败本身（invalid_credentials）不触发。 */
export function setOnUnauthorized(fn: (() => void) | null) {
  onUnauthorized = fn
}

/** 注册全局 403 处理(App 设为:admin 路径下 toast + 回前台)。 */
export function setOnForbidden(fn: (() => void) | null) {
  onForbidden = fn
}

/** 供非 fetch 通道（XHR 上传）复用全局 401 处理。 */
export function notifyUnauthorized() {
  onUnauthorized?.()
}

export async function api<T = unknown>(path: string, init: RequestInit = {}): Promise<T> {
  let res: Response
  try {
    res = await fetch('/api/v1' + path, init)
  } catch {
    throw new ApiError(0, 'network_error', t('errors.network_error'))
  }
  let env: Envelope<T>
  try {
    env = (await res.json()) as Envelope<T>
  } catch {
    throw new ApiError(res.status, 'internal_error', t('errors.responseFormat'))
  }
  if (!res.ok || !env.status) {
    const code = (env.data as { code?: string } | null)?.code ?? 'internal_error'
    // 会话探测的 401 是「未登录」的正常答案(游客模式下 / 也会探测),不触发全局跳登录;
    // RequireAuth/RequireAdmin 自行 Navigate。会话中途失效仍由业务请求的 401 兜住。
    if (res.status === 401 && code !== 'invalid_credentials' && path !== '/auth/session') onUnauthorized?.()
    if (res.status === 403) onForbidden?.()
    throw new ApiError(res.status, code, env.message || t('errors.requestFailed'), parseRetryAfter(res))
  }
  return env.data
}

function withBody<T>(method: string) {
  return (path: string, body?: unknown): Promise<T> =>
    api<T>(path, {
      method,
      ...(body === undefined
        ? {}
        : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
    })
}

export const post = <T = unknown>(path: string, body?: unknown) => withBody<T>('POST')(path, body)
export const put = <T = unknown>(path: string, body?: unknown) => withBody<T>('PUT')(path, body)
export const patch = <T = unknown>(path: string, body?: unknown) => withBody<T>('PATCH')(path, body)
export const del = <T = unknown>(path: string, body?: unknown) => withBody<T>('DELETE')(path, body)
