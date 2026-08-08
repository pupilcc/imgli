import type { PublicAlbumImg } from '../../api/types'
import { buildAlbumSearch, type AlbumPublicMode } from './albumPublicView'

export function albumPageURL(
  id: string | number,
  mode: AlbumPublicMode = 'gallery',
  index0 = 0,
): string {
  const base = `${window.location.origin}/a/${id}`
  const q = buildAlbumSearch(mode, index0)
  return q ? `${base}?${q}` : base
}

export function sharePageURL(img: Pick<PublicAlbumImg, 'key' | 'share_path'>): string {
  const path = img.share_path || `/s/${img.key}`
  return `${window.location.origin}${path}`
}

export function aspectStyle(img: Pick<PublicAlbumImg, 'width' | 'height'>): { aspectRatio: string } {
  const w = img.width > 0 ? img.width : 1
  const h = img.height > 0 ? img.height : 1
  return { aspectRatio: `${w} / ${h}` }
}
