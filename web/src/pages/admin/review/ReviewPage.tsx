import { useAdminReview, useReviewBatch, useReviewDecide } from '../../../api/adminHooks'
import { useT } from '../../../i18n'
import { formatBytes } from '../../../lib/format'
import { useAdminSearchParam } from '../../../lib/useAdminSearchParam'
import { PageHeader } from '../../../shell/PageHeader'
import { useGlobal } from '../../../store'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { Tag } from '../../../ui/Tag'
import { AdminQueryGate } from '../ui/AdminQueryGate'
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
  const { params, setParam } = useAdminSearchParam()
  const page = Number(params.get('page')) || 1
  const q = useAdminReview(page)
  const decide = useReviewDecide()
  const batch = useReviewBatch()

  const setPage = (p: number) => setParam('page', p > 1 ? String(p) : '')

  const busy = decide.isPending || batch.isPending

  return (
    <div>
      <PageHeader
        kicker="REVIEW QUEUE"
        title={t('adminA.reviewTitle')}
        extra={
          (q.data?.items.length ?? 0) > 0 ? (
            <Button
              variant="primary"
              disabled={busy}
              onClick={() =>
                batch.mutate(
                  { keys: (q.data?.items ?? []).map((i) => i.key), action: 'approve' },
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
              {t('adminA.approveAll', { count: q.data?.items.length ?? 0 })}
            </Button>
          ) : undefined
        }
      />
      <AdminQueryGate query={q}>
        {(data) => {
          const items = data.items
          return items.length === 0 ? (
            data.total > 0 ? (
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
          <p className={styles.stat}>{t('adminA.pendingTotal', { total: data.total })}</p>
          <Pager page={page} limit={data.limit} total={data.total} onPage={setPage} />
        </>
          )
        }}
      </AdminQueryGate>
    </div>
  )
}
