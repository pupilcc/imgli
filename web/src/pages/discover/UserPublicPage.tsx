import { useState } from 'react'
import { useParams } from 'react-router'
import { ApiError } from '../../api/client'
import { useUserImages, useUserPublic } from '../../api/hooks'
import type { DiscoverRow } from '../../api/types'
import { useT } from '../../i18n'
import { EmptyState } from '../../ui/EmptyState'
import { Segmented } from '../../ui/Segmented'
import { ImageCard } from './ImageCard'
import { Lightbox } from './Lightbox'

const moreBtn =
  'h-8 cursor-pointer rounded-sm border border-border bg-surface px-[18px] text-sm-plus font-semibold text-ink hover:enabled:bg-soft disabled:cursor-default disabled:opacity-55'
const centerMsg = 'px-4 py-20 text-center text-[13px] text-muted'

/** 用户公开主页：资料头 + 公开图网格 + 灯箱。 */
export function UserPublicPage() {
  const { t, lang } = useT()
  const { username = '' } = useParams()
  const [sort, setSort] = useState<'new' | 'hot'>('new')
  const prof = useUserPublic(username)
  const imgs = useUserImages(username, sort)
  const rows = imgs.data?.pages.flatMap((p) => p.items) ?? []
  const [active, setActive] = useState<DiscoverRow | null>(null)

  const closed = prof.error instanceof ApiError && prof.error.httpStatus === 404
  const user = prof.data?.user

  const sortOptions = [
    { value: 'new' as const, label: t('discover.sortNew') },
    { value: 'hot' as const, label: t('discover.sortHot') },
  ]

  if (closed) {
    return <div className={centerMsg}>{t('discover.profileNotFound')}</div>
  }

  if (prof.isLoading || !user) {
    return <div className={centerMsg}>{t('discover.loading')}</div>
  }

  const displayName = user.nickname || user.username
  const initial = (displayName[0] || '?').toUpperCase()
  const joined = new Date(user.joined_at).toLocaleDateString(lang === 'zh' ? 'zh-CN' : 'en-US')

  return (
    <div className="flex flex-col gap-5">
      <header className="flex items-center gap-4 py-2 pb-1">
        <div
          className="flex h-14 w-14 flex-none items-center justify-center rounded-full border border-border bg-soft text-xl font-bold text-muted"
          aria-hidden
        >
          {initial}
        </div>
        <div className="min-w-0">
          <div className="text-lg font-bold leading-snug">{displayName}</div>
          <div className="mt-0.5 font-mono text-xs text-muted">@{user.username}</div>
          <div className="mt-2 flex flex-wrap items-center gap-1.5 text-xs text-muted">
            <span>{t('discover.joinedAt', { date: joined })}</span>
            <span className="opacity-50">·</span>
            <span>{t('discover.publicCount', { count: user.public_image_count })}</span>
          </div>
        </div>
      </header>

      <div className="flex justify-end">
        <Segmented<'new' | 'hot'> options={sortOptions} value={sort} onChange={setSort} />
      </div>

      {imgs.isLoading ? (
        <div className={centerMsg}>{t('discover.loading')}</div>
      ) : rows.length === 0 ? (
        <EmptyState title={t('discover.emptyPublic')} />
      ) : (
        <>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-3.5">
            {rows.map((r) => (
              <ImageCard key={r.key} row={r} onOpen={setActive} />
            ))}
          </div>
          {imgs.hasNextPage && (
            <div className="flex justify-center py-2 pb-4">
              <button
                type="button"
                className={moreBtn}
                disabled={imgs.isFetchingNextPage}
                onClick={() => imgs.fetchNextPage()}
              >
                {imgs.isFetchingNextPage ? t('discover.loading') : t('discover.loadMore')}
              </button>
            </div>
          )}
        </>
      )}

      <Lightbox row={active} onClose={() => setActive(null)} />
    </div>
  )
}
