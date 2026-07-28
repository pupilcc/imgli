import { useSearchParams } from 'react-router'
import { useAdminReview, useReviewBatch, useReviewDecide } from '../../../api/adminHooks'
import { useT } from '../../../i18n'
import { formatBytes } from '../../../lib/format'
import { PageHeader } from '../../../shell/PageHeader'
import { useGlobal } from '../../../store'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { Skeleton } from '../../../ui/Skeleton'
import { Tag } from '../../../ui/Tag'
import { AdminError } from '../ui/AdminError'
import { Pager } from '../ui/Pager'
import styles from './ReviewPage.module.css'

function nsfwVariant(score: number | null): 'ok' | 'warn' | 'err' | 'muted' {
  if (score == null) return 'muted'
  if (score >= 0.8) return 'err'
  if (score >= 0.5) return 'warn'
  return 'ok'
}

export function ReviewPage() {
  const { t } = useT()
  const [params, setParams] = useSearchParams()
  const page = Number(params.get('page')) || 1
  const q = useAdminReview(page)
  const decide = useReviewDecide()
  const batch = useReviewBatch()

  const setPage = (p: number) =>
    setParams((prev) => {
      const n = new URLSearchParams(prev)
      if (p > 1) n.set('page', String(p))
      else n.delete('page')
      return n
    })

  const items = q.data?.items ?? []
  const busy = decide.isPending || batch.isPending

  return (
    <div>
      <PageHeader
        kicker="REVIEW QUEUE"
        title={t('adminA.reviewTitle')}
        extra={
          items.length > 0 ? (
            <Button
              variant="primary"
              disabled={busy}
              onClick={() =>
                batch.mutate(
                  { keys: items.map((i) => i.key), action: 'approve' },
                  {
                    onSuccess: (data) => {
                      const ok = data.results.filter((r) => r.ok).length
                      const failed = data.results.length - ok
                      if (failed > 0) useGlobal.getState().pushToast(t('adminA.batchPartial', { ok, failed }))
                    },
                  },
                )
              }
            >
              {t('adminA.approveAll', { count: items.length })}
            </Button>
          ) : undefined
        }
      />
      {q.isError ? (
        <AdminError onRetry={() => q.refetch()} />
      ) : !q.data ? (
        <Skeleton height={220} />
      ) : items.length === 0 ? (
        q.data.total > 0 ? (
          <EmptyState badge="✓" title={t('adminA.pageCleared')} desc={t('adminA.pageClearedReviewDesc')}>
            <Button variant="secondary" onClick={() => setPage(1)}>{t('adminA.backToPage1')}</Button>
          </EmptyState>
        ) : (
          <EmptyState badge="✓" title="ALL CLEAR" desc={t('adminA.allClearDesc')} />
        )
      ) : (
        <>
          <div className={styles.stream}>
            {items.map((it) => (
              <div key={it.key} className={styles.card}>
                <img className={styles.img} src={it.links.thumbnail_url} alt={it.name} loading="lazy" />
                <div className={styles.info}>
                  <div className={styles.name}>{it.name}</div>
                  <div className={styles.sub}>
                    {it.username}（#{it.user_id}） · {formatBytes(it.size)}
                  </div>
                  <div className={styles.tagRow}>
                    <Tag variant={nsfwVariant(it.nsfw_score)}>
                      NSFW {it.nsfw_score == null ? '—' : it.nsfw_score.toFixed(2)}
                    </Tag>
                  </div>
                  {it.triggers && it.triggers.length > 0 ? (
                    <div className={styles.triggers} title={t('adminA.triggerReasons')}>
                      {it.triggers.map((tr, i) => {
                        const bits = [tr.plugin, tr.severity]
                        if (tr.score != null) bits.push(tr.score.toFixed(2))
                        if (tr.hits?.length) bits.push(tr.hits.join(','))
                        return (
                          <span key={`${tr.plugin}-${i}`} className={styles.triggerChip}>
                            {bits.join(' · ')}
                          </span>
                        )
                      })}
                    </div>
                  ) : null}
                  <div className={styles.acts}>
                    <Button variant="primary" disabled={busy} onClick={() => decide.mutate({ key: it.key, action: 'approve' })}>
                      {t('adminA.approve')}
                    </Button>
                    <Button variant="danger" disabled={busy} onClick={() => decide.mutate({ key: it.key, action: 'reject' })}>
                      {t('adminA.reject')}
                    </Button>
                  </div>
                </div>
              </div>
            ))}
          </div>
          <p className={styles.stat}>{t('adminA.pendingTotal', { total: q.data.total })}</p>
          <Pager page={page} limit={q.data.limit} total={q.data.total} onPage={setPage} />
        </>
      )}
    </div>
  )
}
