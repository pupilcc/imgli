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
import styles from './UserPublicPage.module.css'

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
    return <div className={styles.centerMsg}>{t('discover.profileNotFound')}</div>
  }

  if (prof.isLoading || !user) {
    return <div className={styles.centerMsg}>{t('discover.loading')}</div>
  }

  const displayName = user.nickname || user.username
  const initial = (displayName[0] || '?').toUpperCase()
  const joined = new Date(user.joined_at).toLocaleDateString(lang === 'zh' ? 'zh-CN' : 'en-US')

  return (
    <div className={styles.page}>
      <header className={styles.profile}>
        <div className={styles.initial} aria-hidden>
          {initial}
        </div>
        <div className={styles.meta}>
          <div className={styles.displayName}>{displayName}</div>
          <div className={styles.handle}>@{user.username}</div>
          <div className={styles.stats}>
            <span>{t('discover.joinedAt', { date: joined })}</span>
            <span className={styles.dot}>·</span>
            <span>{t('discover.publicCount', { count: user.public_image_count })}</span>
          </div>
        </div>
      </header>

      <div className={styles.toolbar}>
        <Segmented<'new' | 'hot'> options={sortOptions} value={sort} onChange={setSort} />
      </div>

      {imgs.isLoading ? (
        <div className={styles.centerMsg}>{t('discover.loading')}</div>
      ) : rows.length === 0 ? (
        <EmptyState title={t('discover.emptyPublic')} />
      ) : (
        <>
          <div className={styles.grid}>
            {rows.map((r) => (
              <ImageCard key={r.key} row={r} onOpen={setActive} />
            ))}
          </div>
          {imgs.hasNextPage && (
            <div className={styles.moreWrap}>
              <button
                type="button"
                className={styles.moreBtn}
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
