import { useState } from 'react'
import { useAdminLogs } from '../../../api/adminHooks'
import type { AuditLog } from '../../../api/types'
import { useT } from '../../../i18n'
import { formatDate } from '../../../lib/format'
import { useAdminSearchParam } from '../../../lib/useAdminSearchParam'
import { PageHeader } from '../../../shell/PageHeader'
import { EmptyState } from '../../../ui/EmptyState'
import { AdminQueryGate } from '../ui/AdminQueryGate'
import { Pager } from '../ui/Pager'
import { actionLabel, actionOptions, actorTypeLabel, ACTOR_TYPE_LABELS, dotColor } from '../ui/auditActions'
import forms from '../ui/adminForms.module.css'
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
  const { params, setParam } = useAdminSearchParam()
  const action = params.get('action') ?? ''
  const actorType = params.get('actor_type') ?? ''
  const page = Number(params.get('page')) || 1
  const [expanded, setExpanded] = useState<number | null>(null)

  const logs = useAdminLogs({ action: action || undefined, actor_type: actorType || undefined, page })
  const options = actionOptions()

  return (
    <div>
      <PageHeader
        kicker="AUDIT LOG"
        title={t('adminB.logsTitle')}
        extra={
          <div className={forms.filters}>
            <select value={action} onChange={(e) => setParam('action', e.target.value)} aria-label={t('adminB.filterActionAria')} className={forms.select}>
              <option value="">{t('adminB.allActions')}</option>
              {options.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
            <select value={actorType} onChange={(e) => setParam('actor_type', e.target.value)} aria-label={t('adminB.filterActorAria')} className={forms.select}>
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
      <AdminQueryGate query={logs}>
        {(data) =>
          data.items.length === 0 ? (
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
                {data.items.map((l) => (
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
                    {expanded === l.id && (
                      <pre className={styles.detail}>{prettyDetail(l.detail, t('adminB.noDetail'))}</pre>
                    )}
                  </div>
                ))}
              </div>
              <p className={styles.stat}>{t('adminB.logsTotal', { total: data.total })}</p>
              <Pager
                page={page}
                limit={data.limit}
                total={data.total}
                onPage={(p) => setParam('page', p > 1 ? String(p) : '')}
              />
            </>
          )
        }
      </AdminQueryGate>
    </div>
  )
}
