import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { api, ApiError } from '../../api/client'
import { useT } from '../../i18n'
import { EmptyState } from '../../ui/EmptyState'
import { Button } from '../../ui/Button'

type AlbumMeta = {
  id: number
  name: string
  visibility: string
  image_count: number
  cover_key: string
}

type AlbumImg = {
  key: string
  name: string
  ext: string
  width: number
  height: number
  size: number
  thumbnail_url: string
  url: string
}

/** 公开相册访客页 /a/:id */
export function PublicAlbumPage() {
  const { id = '' } = useParams()
  const { t } = useT()
  const [active, setActive] = useState<AlbumImg | null>(null)

  const meta = useQuery({
    queryKey: ['public-album', id],
    enabled: !!id,
    retry: false,
    queryFn: () => api<AlbumMeta>(`/a/${id}`),
  })

  const imgs = useInfiniteQuery({
    queryKey: ['public-album-imgs', id],
    enabled: !!id && !!meta.data,
    initialPageParam: '' as string,
    queryFn: ({ pageParam }) => {
      const q = pageParam ? `?cursor=${encodeURIComponent(pageParam)}` : ''
      return api<{ items: AlbumImg[]; next_cursor: string }>(`/a/${id}/images${q}`)
    },
    getNextPageParam: (last) => last.next_cursor || undefined,
  })

  const notFound = meta.error instanceof ApiError && meta.error.httpStatus === 404
  const rows = imgs.data?.pages.flatMap((p) => p.items) ?? []

  if (meta.isLoading) {
    return <div className="flex flex-col items-center gap-4 px-4 py-12 text-center text-muted">{t('discover.loading')}</div>
  }
  if (notFound) {
    return (
      <div className="flex flex-col items-center gap-4 px-4 py-12 text-center text-muted">
        <EmptyState title={t('albums.publicNotFound')} />
        <Link to="/">
          <Button variant="primary">{t('share.uploadCta')}</Button>
        </Link>
      </div>
    )
  }
  if (meta.isError || !meta.data) {
    return <div className="flex flex-col items-center gap-4 px-4 py-12 text-center text-muted">{t('share.loadFailed')}</div>
  }

  return (
    <div className="mx-auto max-w-[1100px] px-4 pt-6 pb-12">
      <header className="mb-5">
        <h1 className="mb-1.5 mt-0 text-[22px] font-bold">{meta.data.name}</h1>
        <p className="m-0 text-[13px] text-muted">{t('albums.publicCount', { count: meta.data.image_count })}</p>
      </header>
      {imgs.isLoading ? (
        <div className="flex flex-col items-center gap-4 px-4 py-12 text-center text-muted">{t('discover.loading')}</div>
      ) : rows.length === 0 ? (
        <EmptyState title={t('albums.publicEmpty')} />
      ) : (
        <>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-2.5">
            {rows.map((r) => (
              <button
                key={r.key}
                type="button"
                className="aspect-square cursor-pointer overflow-hidden rounded border border-border bg-soft p-0"
                onClick={() => setActive(r)}
              >
                <img src={r.thumbnail_url} alt={r.name} loading="lazy" className="block size-full object-cover" />
              </button>
            ))}
          </div>
          {imgs.hasNextPage && (
            <div className="mt-5 flex justify-center">
              <Button variant="secondary" disabled={imgs.isFetchingNextPage} onClick={() => imgs.fetchNextPage()}>
                {t('discover.loadMore')}
              </Button>
            </div>
          )}
        </>
      )}
      {active && (
        <div
          className="fixed inset-0 z-[80] flex items-center justify-center bg-black/72 p-6"
          role="dialog"
          onClick={() => setActive(null)}
        >
          <img
            src={active.url}
            alt={active.name}
            className="max-h-[90vh] max-w-[min(96vw,1100px)] rounded object-contain"
            onClick={(e) => e.stopPropagation()}
          />
        </div>
      )}
    </div>
  )
}
