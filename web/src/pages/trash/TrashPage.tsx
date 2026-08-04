import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router'
import { useEmptyTrash, usePurgeImage, useRestoreImage, useTrash } from '../../api/hooks'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { formatBytes, formatDate } from '../../lib/format'
import { useGlobal } from '../../store'
import { Button } from '../../ui/Button'
import { EmptyState } from '../../ui/EmptyState'
import { InlineConfirm } from '../../ui/InlineConfirm'
import { Modal } from '../../ui/Modal'

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
    <div className="mx-auto max-w-[1120px] pt-11">
      <div className="mb-5 flex items-end justify-between border-b border-border pb-[18px]">
        <div>
          <Link
            to="/images"
            className="mb-2 inline-block font-mono text-[11px] tracking-[0.14em] text-muted hover:text-ink"
          >
            ← LIBRARY
          </Link>
          <h1 className="m-0 text-[26px] font-bold tracking-[-0.015em]">{t('trash.title')}</h1>
        </div>
        {items.length > 0 && (
          <Button variant="danger" onClick={() => setShowEmpty(true)}>
            {t('trash.emptyTrash')}
          </Button>
        )}
      </div>

      <div className="mb-6 flex items-center gap-2.5 rounded-sm border border-border bg-surface px-4 py-[11px]">
        <span className="shrink-0 rounded-[2px] border border-border px-[7px] py-0.5 font-mono text-[9.5px] tracking-[0.1em] text-muted">
          TRASH
        </span>
        <span className="text-sm-plus text-muted">
          {t('trash.retainBefore')}
          <strong className="text-ink">{t('trash.retainDays')}</strong>
          {t('trash.retainAfter')}
        </span>
        <span className="ml-auto shrink-0 font-mono text-xs-plus text-muted">
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
          <div className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-3.5">
            {items.map((i) => (
              <div
                key={i.key}
                className="animate-[rise_0.28s_both] overflow-hidden rounded-sm border border-border bg-surface"
              >
                <div className="relative">
                  <div className="stripe flex aspect-[4/3] items-center justify-center opacity-60">
                    <span className="font-mono text-2xs tracking-[0.06em] text-muted">{i.ext.toUpperCase()}</span>
                  </div>
                  <span
                    className={cn(
                      'absolute bottom-2.5 left-2.5 rounded-[2px] border border-border bg-surface px-2 py-0.5 font-mono text-[9.5px] tracking-[0.06em] text-muted',
                      i.days_left <= 3 && 'urgent border-err bg-err text-white',
                    )}
                  >
                    {t('trash.daysLeft', { days: i.days_left })}
                  </span>
                </div>
                <div className="border-t border-border px-3 py-2.5">
                  <div className="mb-[3px] flex items-baseline gap-2">
                    <span className="min-w-0 flex-1 truncate font-mono text-xs-plus text-ink">{i.name}</span>
                    <span className="shrink-0 font-mono text-2xs text-muted">{formatBytes(i.size)}</span>
                  </div>
                  <div className="font-mono text-2xs text-muted">
                    {t('trash.deletedAt', { date: formatDate(i.deleted_at) })}
                  </div>
                  <div className="mt-2.5 flex gap-1.5 [&>*]:flex-1 [&_button]:px-0 [&_button]:py-1.5 [&_button]:text-[11.5px] [&_button]:font-bold">
                    <Button
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
          <div ref={sentinelRef} className="h-px" />
        </>
      )}

      <Modal open={showEmpty} onClose={() => setShowEmpty(false)}>
        <div className="mb-1.5 font-mono text-2xs tracking-[0.14em] text-err">EMPTY TRASH</div>
        <div className="text-base font-bold">{t('trash.emptyConfirmTitle')}</div>
        <p className="my-2 mb-4 text-sm-plus leading-relaxed text-muted">
          {t('trash.emptyConfirmDesc', {
            count: `${items.length}${hasNextPage ? '+' : ''}`,
            size: `${formatBytes(totalSize)}${hasNextPage ? t('trash.sizeMore') : ''}`,
          })}
        </p>
        <div className="flex gap-2">
          <Button className="flex-1 py-2.5" onClick={() => setShowEmpty(false)}>
            {t('trash.cancel')}
          </Button>
          <button
            type="button"
            className="flex-1 cursor-pointer rounded-sm border-0 bg-err py-2.5 text-sm-plus font-bold text-white hover:opacity-85 disabled:cursor-not-allowed disabled:opacity-50"
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
