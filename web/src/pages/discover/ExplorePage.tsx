import { useState } from 'react'
import { Link } from 'react-router'
import { ApiError } from '../../api/client'
import { usePlaza, usePlazaAlbums, useSession } from '../../api/hooks'
import type { DiscoverRow } from '../../api/types'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { Button } from '../../ui/Button'
import { EmptyState } from '../../ui/EmptyState'
import { Segmented } from '../../ui/Segmented'
import { AlbumCard } from './AlbumCard'
import { ImageCard } from './ImageCard'
import { Lightbox } from './Lightbox'

const moreBtn =
  'h-8 cursor-pointer rounded-sm border border-border bg-surface px-[18px] text-sm-plus font-semibold text-ink hover:enabled:bg-soft disabled:cursor-default disabled:opacity-55'

/** 广场公开流：图片 / 相册 Tab + 排序 + 网格。 */
export function ExplorePage() {
  const { t } = useT()
  const { data: me } = useSession()
  const [tab, setTab] = useState<'images' | 'albums'>('images')
  const [sort, setSort] = useState<'new' | 'hot'>('new')
  const q = usePlaza(sort)
  const albumsQ = usePlazaAlbums()
  const rows = q.data?.pages.flatMap((p) => p.items) ?? []
  const albumRows = albumsQ.data?.pages.flatMap((p) => p.items) ?? []
  const [active, setActive] = useState<DiscoverRow | null>(null)

  const closed =
    (tab === 'images' ? q.error : albumsQ.error) instanceof ApiError &&
    (tab === 'images' ? q.error : albumsQ.error) instanceof ApiError &&
    ((tab === 'images' ? q.error : albumsQ.error) as ApiError).httpStatus === 404
  const showOptInCta = !!me && !me.public_profile

  const sortOptions = [
    { value: 'new' as const, label: t('discover.sortNew') },
    { value: 'hot' as const, label: t('discover.sortHot') },
  ]
  const tabOptions = [
    { value: 'images' as const, label: t('discover.tabImages') },
    { value: 'albums' as const, label: t('discover.tabAlbums') },
  ]

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="m-0 text-lg font-bold tracking-[0.02em]">{t('discover.title')}</h1>
        <div className="flex flex-wrap items-center gap-2">
          <Segmented<'images' | 'albums'> options={tabOptions} value={tab} onChange={setTab} />
          {tab === 'images' && !closed && (
            <Segmented<'new' | 'hot'> options={sortOptions} value={sort} onChange={setSort} />
          )}
        </div>
      </div>

      {closed ? (
        <div className="px-4 py-20 text-center text-[13px] text-muted">{t('discover.plazaClosed')}</div>
      ) : tab === 'albums' ? (
        albumsQ.isLoading ? (
          <div className="px-4 py-20 text-center text-[13px] text-muted">{t('discover.loading')}</div>
        ) : albumRows.length === 0 ? (
          <EmptyState title={t('discover.emptyAlbums')} desc={t('discover.emptyAlbumsDesc')}>
            {showOptInCta ? (
              <Link to="/settings/profile">
                <Button variant="primary">{t('discover.emptyPublicCta')}</Button>
              </Link>
            ) : null}
          </EmptyState>
        ) : (
          <>
            <div className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-3.5">
              {albumRows.map((c) => (
                <AlbumCard key={c.id} card={c} />
              ))}
            </div>
            {albumsQ.hasNextPage && (
              <div className="flex justify-center py-2 pb-4">
                <button
                  type="button"
                  className={cn(moreBtn)}
                  disabled={albumsQ.isFetchingNextPage}
                  onClick={() => albumsQ.fetchNextPage()}
                >
                  {albumsQ.isFetchingNextPage ? t('discover.loading') : t('discover.loadMore')}
                </button>
              </div>
            )}
          </>
        )
      ) : q.isLoading ? (
        <div className="px-4 py-20 text-center text-[13px] text-muted">{t('discover.loading')}</div>
      ) : rows.length === 0 ? (
        <EmptyState title={t('discover.emptyPublic')} desc={t('discover.emptyPublicDesc')}>
          {showOptInCta ? (
            <Link to="/settings/profile">
              <Button variant="primary">{t('discover.emptyPublicCta')}</Button>
            </Link>
          ) : null}
        </EmptyState>
      ) : (
        <>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-3.5">
            {rows.map((r) => (
              <ImageCard key={r.key} row={r} onOpen={setActive} />
            ))}
          </div>
          {q.hasNextPage && (
            <div className="flex justify-center py-2 pb-4">
              <button
                type="button"
                className={cn(moreBtn)}
                disabled={q.isFetchingNextPage}
                onClick={() => q.fetchNextPage()}
              >
                {q.isFetchingNextPage ? t('discover.loading') : t('discover.loadMore')}
              </button>
            </div>
          )}
        </>
      )}

      <Lightbox row={active} onClose={() => setActive(null)} />
    </div>
  )
}
