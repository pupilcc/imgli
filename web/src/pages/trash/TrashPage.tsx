import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router'
import { useEmptyTrash, usePurgeImage, useRestoreImage, useTrash } from '../../api/hooks'
import { useT } from '../../i18n'
import { formatBytes, formatDate } from '../../lib/format'
import { useGlobal } from '../../store'
import { Button } from '../../ui/Button'
import { EmptyState } from '../../ui/EmptyState'
import { InlineConfirm } from '../../ui/InlineConfirm'
import { Modal } from '../../ui/Modal'
import styles from './TrashPage.module.css'

export function TrashPage() {
  const { t } = useT()
  const trash = useTrash()
  const restore = useRestoreImage()
  const purge = usePurgeImage()
  const empty = useEmptyTrash()
  const pushToast = useGlobal((s) => s.pushToast)
  const [showEmpty, setShowEmpty] = useState(false)
  const sentinelRef = useRef<HTMLDivElement>(null)

  const items = useMemo(() => trash.data?.pages.flatMap((p) => p.items) ?? [], [trash.data])
  const totalSize = useMemo(() => items.reduce((s, i) => s + i.size, 0), [items])

  const { hasNextPage, isFetchingNextPage, fetchNextPage } = trash
  useEffect(() => {
    const el = sentinelRef.current
    if (!el) return
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting) && hasNextPage && !isFetchingNextPage) fetchNextPage()
    })
    io.observe(el)
    return () => io.disconnect()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage, items.length])

  return (
    <div className={styles.page}>
      <div className={styles.head}>
        <div>
          <Link to="/images" className={styles.back}>
            ← LIBRARY
          </Link>
          <h1 className={styles.title}>{t('trash.title')}</h1>
        </div>
        {items.length > 0 && (
          <Button variant="danger" onClick={() => setShowEmpty(true)}>
            {t('trash.emptyTrash')}
          </Button>
        )}
      </div>

      <div className={styles.infoBar}>
        <span className={styles.infoTag}>TRASH</span>
        <span className={styles.infoText}>
          {t('trash.retainBefore')}
          <strong className={styles.infoStrong}>{t('trash.retainDays')}</strong>
          {t('trash.retainAfter')}
        </span>
        <span className={styles.infoStat}>
          {t('trash.stat', {
            count: items.length,
            plus: hasNextPage ? '+' : '',
            size: formatBytes(totalSize),
          })}
        </span>
      </div>

      {trash.isLoading ? null : items.length === 0 ? (
        <EmptyState title={t('trash.emptyTitle')} desc={t('trash.emptyDesc')}>
          <Link to="/images">
            <Button variant="primary">{t('trash.backToImages')}</Button>
          </Link>
        </EmptyState>
      ) : (
        <>
          <div className={styles.grid}>
            {items.map((i) => (
              <div key={i.key} className={styles.card}>
                <div className={styles.thumbWrap}>
                  <div className={`stripe ${styles.thumb}`}>
                    <span className={styles.ext}>{i.ext.toUpperCase()}</span>
                  </div>
                  <span className={[styles.days, i.days_left <= 3 && styles.urgent].filter(Boolean).join(' ')}>
                    {t('trash.daysLeft', { days: i.days_left })}
                  </span>
                </div>
                <div className={styles.meta}>
                  <div className={styles.nameRow}>
                    <span className={styles.name}>{i.name}</span>
                    <span className={styles.size}>{formatBytes(i.size)}</span>
                  </div>
                  <div className={styles.deletedAt}>{t('trash.deletedAt', { date: formatDate(i.deleted_at) })}</div>
                  <div className={styles.actions}>
                    <Button
                      className={styles.actionBtn}
                      disabled={restore.isPending}
                      onClick={() =>
                        restore.mutate(i.key, { onSuccess: () => pushToast(t('trash.restored')) })
                      }
                    >
                      {t('trash.restore')}
                    </Button>
                    <InlineConfirm
                      label={t('trash.purge')}
                      disabled={purge.isPending}
                      onConfirm={() => purge.mutate(i.key, { onSuccess: () => pushToast(t('trash.purged')) })}
                    />
                  </div>
                </div>
              </div>
            ))}
          </div>
          <div ref={sentinelRef} className={styles.sentinel} />
        </>
      )}

      <Modal open={showEmpty} onClose={() => setShowEmpty(false)}>
        <div className={styles.modalKicker}>EMPTY TRASH</div>
        <div className={styles.modalTitle}>{t('trash.emptyConfirmTitle')}</div>
        <p className={styles.modalDesc}>
          {t('trash.emptyConfirmDesc', {
            count: `${items.length}${hasNextPage ? '+' : ''}`,
            size: `${formatBytes(totalSize)}${hasNextPage ? t('trash.sizeMore') : ''}`,
          })}
        </p>
        <div className={styles.modalRow}>
          <Button className={styles.modalBtn} onClick={() => setShowEmpty(false)}>
            {t('trash.cancel')}
          </Button>
          <button
            type="button"
            className={styles.emptyBtn}
            disabled={empty.isPending}
            onClick={() =>
              empty.mutate(undefined, {
                onSuccess: (d) => {
                  setShowEmpty(false)
                  pushToast(t('trash.emptied', { count: d.purged }))
                },
              })
            }
          >
            {t('trash.purgeAll')}
          </button>
        </div>
      </Modal>
    </div>
  )
}
