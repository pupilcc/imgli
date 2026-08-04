import { useState } from 'react'
import { useAdminLogs } from '../../../api/adminHooks'
import type { AuditLog } from '../../../api/types'
import { useT } from '../../../i18n'
import { cn } from '../../../lib/cn'
import { formatDate } from '../../../lib/format'
import { useAdminSearchParam } from '../../../lib/useAdminSearchParam'
import { PageHeader } from '../../../shell/PageHeader'
import { EmptyState } from '../../../ui/EmptyState'
import { AdminFilters, AdminSelect, AdminTable } from '../ui/adminChrome'
import { AdminQueryGate } from '../ui/AdminQueryGate'
import { Pager } from '../ui/Pager'
import { actionLabel, actionOptions, actorTypeLabel, ACTOR_TYPE_LABELS, dotColor } from '../ui/auditActions'

const COLS = 'minmax(140px,180px) minmax(120px,160px) 1fr minmax(100px,130px)'

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
          <AdminFilters>
            <AdminSelect
              value={action}
              onChange={(e) => setParam('action', e.target.value)}
              aria-label={t('adminB.filterActionAria')}
            >
              <option value="">{t('adminB.allActions')}</option>
              {options.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </AdminSelect>
            <AdminSelect
              value={actorType}
              onChange={(e) => setParam('actor_type', e.target.value)}
              aria-label={t('adminB.filterActorAria')}
            >
              <option value="">{t('adminB.allActors')}</option>
              {Object.keys(ACTOR_TYPE_LABELS).map((value) => (
                <option key={value} value={value}>
                  {actorTypeLabel(value)}
                </option>
              ))}
            </AdminSelect>
          </AdminFilters>
        }
      />
      <AdminQueryGate query={logs}>
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState title={t('adminB.noMatchingLogs')} desc={t('adminB.noMatchingLogsDesc')} />
          ) : (
            <>
              <AdminTable className="mt-2 overflow-hidden rounded-lg" minWidth={560}>
                <div
                  className="grid items-center gap-3 border-b border-border bg-soft px-3.5 py-2.5 text-xs text-muted"
                  style={{ gridTemplateColumns: COLS }}
                >
                  <span>{t('adminB.colTime')}</span>
                  <span>{t('adminB.colActor')}</span>
                  <span className="max-md:hidden">{t('adminB.colType')}</span>
                  <span className="max-md:hidden">{t('adminB.colIp')}</span>
                </div>
                {data.items.map((l) => (
                  <div key={l.id} className="border-b border-border last:border-b-0">
                    <button
                      type="button"
                      className={cn(
                        'grid w-full cursor-pointer items-center gap-3 border-0 bg-transparent px-3.5 py-2.5 text-left font-inherit text-ink',
                      )}
                      style={{ gridTemplateColumns: COLS }}
                      aria-expanded={expanded === l.id}
                      onClick={() => setExpanded(expanded === l.id ? null : l.id)}
                    >
                      <span className="font-variant-numeric tabular-nums text-muted">{formatDate(l.created_at)}</span>
                      <span className="min-w-0 truncate">{actorText(l)}</span>
                      <span className="flex min-w-0 items-center gap-2 max-md:col-span-2">
                        <span className="h-2 w-2 flex-none rounded-full" style={{ background: dotColor(l.action) }} />
                        <span className="min-w-0 truncate">{actionLabel(l.action)}</span>
                        <span className="flex-none text-muted">{expanded === l.id ? '▾' : '▸'}</span>
                      </span>
                      <span className="font-mono text-xs text-muted max-md:hidden">{l.ip || '—'}</span>
                    </button>
                    {expanded === l.id && (
                      <pre className="m-0 overflow-x-auto border-t border-border bg-soft px-3.5 py-3 font-mono text-xs leading-relaxed text-muted whitespace-pre-wrap">
                        {prettyDetail(l.detail, t('adminB.noDetail'))}
                      </pre>
                    )}
                  </div>
                ))}
              </AdminTable>
              <p className="mt-2.5 font-mono text-xs-plus text-muted">{t('adminB.logsTotal', { total: data.total })}</p>
              <Pager page={page} limit={data.limit} total={data.total} onPage={(p) => setParam('page', p > 1 ? String(p) : '')} />
            </>
          )
        }
      </AdminQueryGate>
    </div>
  )
}
