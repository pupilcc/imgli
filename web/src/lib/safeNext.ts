/**
 * 登录/注册成功后的站内回跳。仅允许相对路径，防 open redirect。
 * 缺省或非法 → fallback（默认 `/` 上传页）。
 */
export function safeNext(raw: string | null | undefined, fallback = '/'): string {
  if (!raw) return fallback
  let path = raw.trim()
  try {
    // 允许编码过的 next
    path = decodeURIComponent(path)
  } catch {
    return fallback
  }
  path = path.trim()
  if (!path.startsWith('/') || path.startsWith('//')) return fallback
  // 协议相对 //evil.com
  if (path.includes('://')) return fallback
  // 不要把用户送回登录/注册死循环
  if (path === '/login' || path.startsWith('/login?') || path.startsWith('/forgot-password') || path.startsWith('/reset-password')) {
    return fallback
  }
  return path
}

/** 构造带 next 的登录链接（当前页非登录时）。 */
export function loginHref(nextPath?: string): string {
  const next = safeNext(nextPath ?? (typeof window !== 'undefined' ? `${window.location.pathname}${window.location.search}` : '/'), '/')
  if (next === '/') return '/login?next=%2F'
  return `/login?next=${encodeURIComponent(next)}`
}
