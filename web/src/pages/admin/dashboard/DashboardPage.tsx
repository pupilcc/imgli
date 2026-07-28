import { useAdminLogs, useAdminStats } from '../../../api/adminHooks'
import { useT } from '../../../i18n'
import { formatBytes, formatDate } from '../../../lib/format'
import { PageHeader } from '../../../shell/PageHeader'
import { Skeleton } from '../../../ui/Skeleton'
import { AdminError } from '../ui/AdminError'
import { ACTION_LABELS, dotColor } from '../ui/auditActions'
import styles from './DashboardPage.module.css'
import { TrendChart } from './TrendChart'

export function DashboardPage() {
  const { t } = useT()
  const stats = useAdminStats()
  const logs = useAdminLogs({ limit: 8 })
  return (
    <div>
      <PageHeader kicker="DASHBOARD" title={t('adminA.dashTitle')} />
      {stats.isError ? (
        <AdminError onRetry={() => stats.refetch()} />
      ) : !stats.data ? (
        <Skeleton height={220} />
      ) : (
        <>
          <div className={styles.cards}>
            {[
              { label: t('adminA.usersCount'), value: String(stats.data.users) },
              { label: t('adminA.imagesCount'), value: String(stats.data.images) },
              { label: t('adminA.totalStorage'), value: formatBytes(stats.data.storage) },
              { label: t('adminA.todayUploads'), value: String(stats.data.today_uploads) },
              { label: t('adminA.pendingImages'), value: String(stats.data.pending_images ?? 0) },
              { label: t('adminA.rejectedImages'), value: String(stats.data.rejected_images ?? 0) },
              { label: t('adminA.tasksPending'), value: String(stats.data.tasks_pending ?? 0) },
              { label: t('adminA.tasksRunning'), value: String(stats.data.tasks_running ?? 0) },
            ].map((c) => (
              <div key={c.label} className={styles.card}>
                <div className={styles.cardLabel}>{c.label}</div>
                <div className={styles.cardValue}>{c.value}</div>
              </div>
            ))}
          </div>
          <div className={styles.panels}>
            <div className={styles.panel}>
              <div className={styles.panelHead}>
                <span>{t('adminA.uploadTrend30d')}</span>
                <span>{t('adminA.unitImagesPerDay')}</span>
              </div>
              <TrendChart daily={stats.data.daily ?? []} />
            </div>
            <div className={styles.panel}>
              <div className={styles.panelHead}>
                <span>{t('adminA.recentEvents')}</span>
              </div>
              <div className={styles.events}>
                {logs.data && logs.data.items.length === 0 && <div className={styles.eventEmpty}>{t('adminA.noEvents')}</div>}
                {logs.data?.items.map((l) => (
                  <div key={l.id} className={styles.event}>
                    <span className={styles.dot} style={{ background: dotColor(l.action) }} />
                    <div className={styles.eventBody}>
                      <div className={styles.eventText}>
                        {ACTION_LABELS[l.action] ?? l.action}
                        {l.actor_id != null && <span className={styles.eventActor}> · #{l.actor_id}</span>}
                      </div>
                      <div className={styles.eventTime}>{formatDate(l.created_at)}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
          <div className={styles.panels}>
            <div className={styles.panel}>
              <div className={styles.panelHead}>
                <span>{t('adminA.traffic7d')}</span>
                <span>{t('adminA.unitViewsPerDay')}</span>
              </div>
              <TrendChart
                daily={(stats.data.traffic_7d ?? []).map((d) => ({ date: d.date, count: d.views }))}
                days={7}
              />
            </div>
            <div className={styles.panel}>
              <div className={styles.panelHead}>
                <span>{t('adminA.topReferers')}</span>
              </div>
              {(stats.data.top_referers ?? []).length === 0 ? (
                <div className={styles.eventEmpty}>{t('adminA.noReferers')}</div>
              ) : (
                <table className={styles.refTable}>
                  <thead>
                    <tr>
                      <th>{t('adminA.domain')}</th>
                      <th>{t('adminA.hits')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(stats.data.top_referers ?? []).map((r) => (
                      <tr key={r.host}>
                        <td>{r.host}</td>
                        <td className={styles.refCount}>{r.count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        </>
      )}
    </div>
  )
}
