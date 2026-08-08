import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { api, ApiError } from '../../api/client'
import { queryKeys } from '../../api/queryKeys'
import type { PublicAlbumImg, PublicAlbumMeta } from '../../api/types'

/** 公开相册元数据 + 无限滚动图片列表。 */
export function usePublicAlbum(id: string) {
  const meta = useQuery({
    queryKey: queryKeys.publicAlbum(id),
    enabled: !!id,
    retry: false,
    queryFn: () => api<PublicAlbumMeta>(`/a/${id}`),
  })

  const imgs = useInfiniteQuery({
    queryKey: queryKeys.publicAlbumImgs(id),
    enabled: !!id && !!meta.data,
    initialPageParam: '' as string,
    queryFn: ({ pageParam }) => {
      const q = pageParam ? `?cursor=${encodeURIComponent(pageParam)}` : ''
      return api<{ items: PublicAlbumImg[]; next_cursor: string }>(`/a/${id}/images${q}`)
    },
    getNextPageParam: (last) => last.next_cursor || undefined,
  })

  const notFound = meta.error instanceof ApiError && meta.error.httpStatus === 404
  const rows = imgs.data?.pages.flatMap((p) => p.items) ?? []

  return { meta, imgs, rows, notFound }
}
