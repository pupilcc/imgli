import { useState } from 'react'
import { ApiError } from '../../api/client'
import { usePlaza } from '../../api/hooks'
import type { DiscoverRow } from '../../api/types'
import { useT } from '../../i18n'
import { EmptyState } from '../../ui/EmptyState'
import { Segmented } from '../../ui/Segmented'
import { ImageCard } from './ImageCard'
import { Lightbox } from './Lightbox'
import styles from './ExplorePage.module.css'

/** 广场公开流：排序 + 网格 + 灯箱。 */
export function ExplorePage() {
  const { t } = useT()
  const [sort, setSort] = useState<'new' | 'hot'>('new')
  const q = usePlaza(sort)
  const rows = q.data?.pages.flatMap((p) => p.items) ?? []
  const [active, setActive] = useState<DiscoverRow | null>(null)

  const closed = q.error instanceof ApiError && q.error.httpStatus === 404

  const sortOptions = [
    { value: 'new' as const, label: t('discover.sortNew') },
    { value: 'hot' as const, label: t('discover.sortHot') },
  ]

  return (
    <div className={styles.page}>
      <div className={styles.head}>
        <h1 className={styles.title}>{t('discover.title')}</h1>
        {!closed && (
          <Segmented<'new' | 'hot'> options={sortOptions} value={sort} onChange={setSort} />
        )}
      </div>

      {closed ? (
        <div className={styles.centerMsg}>{t('discover.plazaClosed')}</div>
      ) : q.isLoading ? (
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
          {q.hasNextPage && (
            <div className={styles.moreWrap}>
              <button
                type="button"
                className={styles.moreBtn}
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
