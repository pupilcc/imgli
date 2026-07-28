import { useState } from 'react'
import { useSearchParams } from 'react-router'
import { useAdminLogs } from '../../../api/adminHooks'
import type { AuditLog } from '../../../api/types'
import { useT } from '../../../i18n'
import { formatDate } from '../../../lib/format'
import { PageHeader } from '../../../shell/PageHeader'
import { EmptyState } from '../../../ui/EmptyState'
import { Skeleton } from '../../../ui/Skeleton'
import { AdminError } from '../ui/AdminError'
import { Pager } from '../ui/Pager'
import { actionLabel, actionOptions, actorTypeLabel, ACTOR_TYPE_LABELS, dotColor } from '../ui/auditActions'
import styles from './LogsPage.module.css'

function prettyDetail(detail: string, emptyLabel: string): string {
  if (!detail) return emptyLabel
  try {
    return JSON.stringify(JSON.parse(detail), null, 2)
  } catch {
    return detail
  }
}

function actorText(l: AuditLog): string {
  const typeLabel = actorTypeLabel(l.actor_type)
  return l.actor_id != null ? `${typeLabel} #${l.actor_id}` : typeLabel
}

export function LogsPage() {
  const { t } = useT()
  const [params, setParams] = useSearchParams()
  const action = params.get('action') ?? ''
  const actorType = params.get('actor_type') ?? ''
  const page = Number(params.get('page')) || 1
  const [expanded, setExpanded] = useState<number | null>(null)

  const setParam = (key: string, value: string) => {
    setParams((p) => {
      const n = new URLSearchParams(p)
      if (value) n.set(key, value)
      else n.delete(key)
      if (key !== 'page') n.delete('page')
      return n
    })
  }

  const logs = useAdminLogs({ action: action || undefined, actor_type: actorType || undefined, page })
  const options = actionOptions()

  return (
    <div>
      <PageHeader
        kicker="AUDIT LOG"
        title={t('adminB.logsTitle')}
        extra={
          <div className={styles.filters}>
            <select value={action} onChange={(e) => setParam('action', e.target.value)} aria-label={t('adminB.filterActionAria')} className={styles.select}>
              <option value="">{t('adminB.allActions')}</option>
              {options.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
            <select value={actorType} onChange={(e) => setParam('actor_type', e.target.value)} aria-label={t('adminB.filterActorAria')} className={styles.select}>
              <option value="">{t('adminB.allActors')}</option>
              {Object.keys(ACTOR_TYPE_LABELS).map((value) => (
                <option key={value} value={value}>
                  {actorTypeLabel(value)}
                </option>
              ))}
            </select>
          </div>
        }
      />
      {logs.isError ? (
        <AdminError onRetry={() => logs.refetch()} />
      ) : !logs.data ? (
        <Skeleton height={220} />
      ) : logs.data.items.length === 0 ? (
        <EmptyState title={t('adminB.noMatchingLogs')} desc={t('adminB.noMatchingLogsDesc')} />
      ) : (
        <>
          <div className={styles.table}>
            <div className={`${styles.head} ${styles.row}`}>
              <span>{t('adminB.colTime')}</span>
              <span>{t('adminB.colActor')}</span>
              <span>{t('adminB.colType')}</span>
              <span>{t('adminB.colIp')}</span>
            </div>
            {logs.data.items.map((l) => (
              <div key={l.id} className={styles.rowWrap}>
                <button
                  type="button"
                  className={styles.row}
                  aria-expanded={expanded === l.id}
                  onClick={() => setExpanded(expanded === l.id ? null : l.id)}
                >
                  <span className={styles.time}>{formatDate(l.created_at)}</span>
                  <span className={styles.actor}>{actorText(l)}</span>
                  <span className={styles.action}>
                    <span className={styles.dot} style={{ background: dotColor(l.action) }} />
                    {actionLabel(l.action)}
                    <span className={styles.caret}>{expanded === l.id ? '▾' : '▸'}</span>
                  </span>
                  <span className={styles.ip}>{l.ip || '—'}</span>
                </button>
                {expanded === l.id && <pre className={styles.detail}>{prettyDetail(l.detail, t('adminB.noDetail'))}</pre>}
              </div>
            ))}
          </div>
          <p className={styles.stat}>{t('adminB.logsTotal', { total: logs.data.total })}</p>
          <Pager page={page} limit={logs.data.limit} total={logs.data.total} onPage={(p) => setParam('page', p > 1 ? String(p) : '')} />
        </>
      )}
    </div>
  )
}
