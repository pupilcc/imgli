import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { api, ApiError } from '../../api/client'
import { useT } from '../../i18n'
import { EmptyState } from '../../ui/EmptyState'
import { Button } from '../../ui/Button'
import styles from './PublicAlbumPage.module.css'

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

  if (meta.isLoading) return <div className={styles.msg}>{t('discover.loading')}</div>
  if (notFound) {
    return (
      <div className={styles.center}>
        <EmptyState title={t('albums.publicNotFound')} />
        <Link to="/">
          <Button variant="primary">{t('share.uploadCta')}</Button>
        </Link>
      </div>
    )
  }
  if (meta.isError || !meta.data) return <div className={styles.msg}>{t('share.loadFailed')}</div>

  return (
    <div className={styles.page}>
      <header className={styles.head}>
        <h1 className={styles.title}>{meta.data.name}</h1>
        <p className={styles.sub}>{t('albums.publicCount', { count: meta.data.image_count })}</p>
      </header>
      {imgs.isLoading ? (
        <div className={styles.msg}>{t('discover.loading')}</div>
      ) : rows.length === 0 ? (
        <EmptyState title={t('albums.publicEmpty')} />
      ) : (
        <>
          <div className={styles.grid}>
            {rows.map((r) => (
              <button key={r.key} type="button" className={styles.card} onClick={() => setActive(r)}>
                <img src={r.thumbnail_url} alt={r.name} loading="lazy" />
              </button>
            ))}
          </div>
          {imgs.hasNextPage && (
            <div className={styles.more}>
              <Button variant="secondary" disabled={imgs.isFetchingNextPage} onClick={() => imgs.fetchNextPage()}>
                {t('discover.loadMore')}
              </Button>
            </div>
          )}
        </>
      )}
      {active && (
        <div className={styles.lb} role="dialog" onClick={() => setActive(null)}>
          <img src={active.url} alt={active.name} onClick={(e) => e.stopPropagation()} />
        </div>
      )}
    </div>
  )
}
