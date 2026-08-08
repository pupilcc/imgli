import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError, post } from '../../api/client'
import { queryKeys } from '../../api/queryKeys'
import type { PublicAlbumImg, PublicAlbumMeta } from '../../api/types'

/** 公开相册元数据 + 无限滚动图片列表（口令未解锁时不拉图）。 */
export function usePublicAlbum(id: string) {
  const qc = useQueryClient()
  const meta = useQuery({
    queryKey: queryKeys.publicAlbum(id),
    enabled: !!id,
    retry: false,
    queryFn: () => api<PublicAlbumMeta>(`/a/${id}`),
  })

  const locked = !!meta.data?.password_required

  const imgs = useInfiniteQuery({
    queryKey: queryKeys.publicAlbumImgs(id),
    enabled: !!id && !!meta.data && !locked,
    initialPageParam: '' as string,
    queryFn: ({ pageParam }) => {
      const q = pageParam ? `?cursor=${encodeURIComponent(pageParam)}` : ''
      return api<{ items: PublicAlbumImg[]; next_cursor: string }>(`/a/${id}/images${q}`)
    },
    getNextPageParam: (last) => last.next_cursor || undefined,
  })

  const unlock = useMutation({
    mutationFn: (password: string) => post<PublicAlbumMeta>(`/a/${id}/unlock`, { password }),
    onSuccess: (data) => {
      qc.setQueryData(queryKeys.publicAlbum(id), data)
      void qc.invalidateQueries({ queryKey: queryKeys.publicAlbumImgs(id) })
    },
  })

  const notFound = meta.error instanceof ApiError && meta.error.httpStatus === 404
  const rows = imgs.data?.pages.flatMap((p) => p.items) ?? []

  return { meta, imgs, rows, notFound, locked, unlock }
}
