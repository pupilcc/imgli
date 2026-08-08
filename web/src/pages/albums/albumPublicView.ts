/** 公开相册视图模式与 URL 查询约定（属主 default_view + URL；无访客 localStorage 偏好）。 */

export type AlbumPublicMode = 'gallery' | 'immersive'

/** 解析 1-based i 参数 → 0-based 下标；非法则 0 */
export function parseIndexParam(raw: string | null): number {
  if (raw == null || raw === '') return 0
  const n = Number.parseInt(raw, 10)
  if (!Number.isFinite(n) || n < 1) return 0
  return n - 1
}

export function buildAlbumSearch(mode: AlbumPublicMode, index0: number): string {
  if (mode !== 'immersive') return ''
  const p = new URLSearchParams()
  p.set('view', 'immersive')
  p.set('i', String(Math.max(1, index0 + 1)))
  return p.toString()
}

export function parseViewParam(raw: string | null): AlbumPublicMode {
  return raw === 'immersive' ? 'immersive' : 'gallery'
}
