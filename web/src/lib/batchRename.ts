/** 与后端 imagesvc batch rename 对齐：可选查找替换 → 可选模板。 */

export function splitFindTerms(find: string): string[] {
  return find
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .split(/[\n|]/)
    .map((s) => s.trim())
    .filter(Boolean)
}

export function replaceAllLiteral(s: string, old: string, repl: string, ignoreCase: boolean): string {
  if (!old) return s
  if (!ignoreCase) return s.split(old).join(repl)
  const lo = old.toLowerCase()
  let out = ''
  let i = 0
  const lower = s.toLowerCase()
  while (i < s.length) {
    if (i + old.length <= s.length && lower.slice(i, i + old.length) === lo) {
      out += repl
      i += old.length
      continue
    }
    out += s[i]
    i += 1
  }
  return out
}

export function cleanNameSeparators(s: string): string {
  let out = ''
  let prevSep = false
  for (const ch of s) {
    const sep = ch === ' ' || ch === '_' || ch === '-'
    if (sep) {
      if (!out || prevSep) continue
      prevSep = true
      out += ch === ' ' ? '_' : ch
      continue
    }
    prevSep = false
    out += ch
  }
  return out.replace(/^[_\-\s]+|[_\-\s]+$/g, '')
}

export function imageBaseName(name: string, ext?: string): string {
  const fromPath = name.replace(/\.[^.]+$/, '')
  if (fromPath) return fromPath
  if (ext) return name.replace(new RegExp(`\\.${ext}$`, 'i'), '')
  return name
}

export function ensureExt(name: string, ext: string): string {
  if (!ext) return name
  if (name.toLowerCase().endsWith(`.${ext.toLowerCase()}`)) return name
  return `${name}.${ext}`
}

export function applyFindReplace(base: string, find: string, replace: string, ignoreCase: boolean): string {
  let out = base
  for (const term of splitFindTerms(find)) {
    out = replaceAllLiteral(out, term, replace, ignoreCase)
  }
  return out
}

export type PatternCtx = {
  name: string
  original: string
  ext: string
  n1: number
  /** RFC3339 or Date-parseable */
  createdAt?: string
  album?: string
}

/** 模板：字面量 + 占位符；{{ → 字面 { */
export function applyPattern(pattern: string, ctx: PatternCtx): string {
  const esc = '\0BRACE\0'
  let out = pattern.split('{{').join(esc)
  out = out.split('{name}').join(ctx.name)
  out = out.split('{original}').join(ctx.original)
  out = out.split('{ext}').join((ctx.ext || '').replace(/^\./, ''))
  out = out.split('{album}').join(ctx.album || '')

  let d = ctx.createdAt ? new Date(ctx.createdAt) : new Date()
  if (Number.isNaN(d.getTime())) d = new Date()
  const yyyy = String(d.getFullYear())
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  out = out.split('{yyyy}').join(yyyy)
  out = out.split('{mm}').join(mm)
  out = out.split('{dd}').join(dd)

  out = out.replace(/\{n(?::(\d+))?\}/g, (_m, pad?: string) => {
    if (pad) {
      const w = Number.parseInt(pad, 10)
      if (Number.isFinite(w) && w > 0 && w <= 8) return String(ctx.n1).padStart(w, '0')
    }
    return String(ctx.n1)
  })
  out = out.split(esc).join('{')
  return out.replace(/\.[^.]+$/, '')
}

export type RenamePreviewStatus = 'ok' | 'unchanged' | 'empty' | 'conflict'

export type RenamePreviewRow = {
  key: string
  from: string
  to: string
  status: RenamePreviewStatus
}

export function computeRename(opts: {
  name: string
  ext: string
  find: string
  replace: string
  ignoreCase: boolean
  cleanSeparators: boolean
  pattern: string
  n1: number
  createdAt?: string
  album?: string
}): string {
  const original = imageBaseName(opts.name, opts.ext)
  let base = original
  const find = opts.find.trim()
  const pattern = opts.pattern.trim()
  if (!find && !pattern) return ''

  if (find) {
    base = applyFindReplace(base, find, opts.replace, opts.ignoreCase)
    if (opts.cleanSeparators) base = cleanNameSeparators(base)
  }
  if (pattern) {
    base = applyPattern(pattern, {
      name: base,
      original,
      ext: opts.ext,
      n1: opts.n1,
      createdAt: opts.createdAt,
      album: opts.album,
    })
    if (opts.cleanSeparators) base = cleanNameSeparators(base)
  }
  base = base.trim().replace(/[/\\]/g, '_')
  if (!base) return ''
  return ensureExt(base, opts.ext)
}

export function previewBatchRename(
  items: { key: string; name: string; ext: string; created_at?: string }[],
  opts: {
    find: string
    replace: string
    ignoreCase: boolean
    cleanSeparators: boolean
    pattern: string
    startN: number
    album?: string
  },
): RenamePreviewRow[] {
  const start = opts.startN >= 1 ? opts.startN : 1
  const rows: RenamePreviewRow[] = items.map((it, i) => {
    const to = computeRename({
      name: it.name,
      ext: it.ext || '',
      find: opts.find,
      replace: opts.replace,
      ignoreCase: opts.ignoreCase,
      cleanSeparators: opts.cleanSeparators,
      pattern: opts.pattern,
      n1: start + i,
      createdAt: it.created_at,
      album: opts.album,
    })
    let status: RenamePreviewStatus = 'ok'
    if (!to) status = 'empty'
    else if (to === it.name) status = 'unchanged'
    return { key: it.key, from: it.name, to: to || '—', status }
  })
  const want = new Map<string, number>()
  for (const r of rows) {
    if (r.status === 'ok') want.set(r.to, (want.get(r.to) || 0) + 1)
  }
  for (const r of rows) {
    if (r.status === 'ok' && (want.get(r.to) || 0) > 1) r.status = 'conflict'
  }
  return rows
}
