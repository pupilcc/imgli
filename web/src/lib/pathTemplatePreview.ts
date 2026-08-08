/** Client-side preview for storage path_template (display only; backend is authoritative). */

const reToken = /\{([^{}]+)\}/g

function fakeRand(kind: string, n: number): string {
  const base62 = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz'
  const hex = '0123456789abcdef'
  const HEX = '0123456789ABCDEF'
  const digits = '0123456789'
  let alphabet = base62
  if (kind === 'hex') alphabet = hex
  else if (kind === 'HEX') alphabet = HEX
  else if (kind === 'digits') alphabet = digits
  let out = ''
  for (let i = 0; i < n; i++) out += alphabet[i % alphabet.length]
  return out
}

function expandToken(inner: string, now: Date, ext: string): string | null {
  switch (inner) {
    case 'Y':
      return String(now.getFullYear())
    case 'm':
      return String(now.getMonth() + 1).padStart(2, '0')
    case 'd':
      return String(now.getDate()).padStart(2, '0')
    case 'H':
      return String(now.getHours()).padStart(2, '0')
    case 'M':
      return String(now.getMinutes()).padStart(2, '0')
    case 'S':
      return String(now.getSeconds()).padStart(2, '0')
    case 'ms':
      return String(now.getMilliseconds()).padStart(3, '0')
    case 'ext':
      return ext.replace(/^\./, '')
    case 'uniqid':
    case 'rand':
      return fakeRand('base62', 12)
    default:
      break
  }
  const colon = inner.indexOf(':')
  if (colon > 0) {
    const name = inner.slice(0, colon)
    const n = Number(inner.slice(colon + 1))
    if (!Number.isFinite(n) || n <= 0) return null
    if (name === 'rand') return fakeRand('base62', n)
    if (name === 'hex') return fakeRand('hex', n)
    if (name === 'HEX') return fakeRand('HEX', n)
    if (name === 'digits') return fakeRand('digits', n)
  }
  return null
}

/** Render a deterministic preview path (not cryptographically random). */
export function previewPathTemplate(tmpl: string, ext = 'png', now = new Date()): string {
  const t = tmpl.trim() || '{Y}/{m}/{d}/{uniqid}.{ext}'
  return t.replace(reToken, (_m, inner: string) => {
    const v = expandToken(inner, now, ext)
    return v ?? `{${inner}}`
  })
}

/** Full example object key: optional prefix + surface + template. */
export function previewObjectKey(opts: {
  prefix?: string
  surface?: 'public' | 'private'
  template: string
  ext?: string
}): string {
  let prefix = (opts.prefix ?? '').trim()
  if (prefix && !prefix.endsWith('/')) prefix += '/'
  const surface = opts.surface ?? 'public'
  const rel = previewPathTemplate(opts.template, opts.ext ?? 'png')
  return `${prefix}${surface}/${rel}`
}

export const PATH_TEMPLATE_PRESETS: { id: string; template: string }[] = [
  { id: 'default', template: '{Y}/{m}/{d}/{uniqid}.{ext}' },
  { id: 'flat', template: '{uniqid}.{ext}' },
  { id: 'toSecond', template: '{Y}/{m}/{d}/{H}{M}{S}_{rand:12}.{ext}' },
  { id: 'digits', template: '{Y}/{m}/{d}/{digits:20}.{ext}' },
]
