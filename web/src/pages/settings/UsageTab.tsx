import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { useQuota } from '../../api/hooks'
import { useT } from '../../i18n'
import { formatBytes } from '../../lib/format'
import { quotaLevel } from '../../ui/QuotaBar'
import { Skeleton } from '../../ui/Skeleton'
import styles from './SettingsPage.module.css'
import own from './UsageTab.module.css'

const levelColor = { ok: 'var(--text)', warn: 'var(--warn)', full: 'var(--err)' } as const

function MeterCard({
  kicker,
  used,
  total,
  note,
  period,
}: {
  kicker: string
  used: number
  total: number
  note: ReactNode
  period?: string
}) {
  const { t } = useT()
  const pct = total > 0 ? Math.min(100, (used / total) * 100) : 0
  const level = quotaLevel(used, total)
  const unlimited = total <= 0
  return (
    <section>
      <div className={styles.kicker}>{kicker}</div>
      <div className={styles.card}>
        <div className={own.bigRow}>
          <div className={own.big}>
            {formatBytes(used)}{' '}
            <span className={own.bigSub}>
              / {unlimited ? t('ui.quotaUnlimited') : formatBytes(total)}
            </span>
          </div>
          {period ? <span className={own.period}>{period}</span> : null}
        </div>
        {!unlimited && (
          <div className={own.track}>
            <div className={own.fill} style={{ width: `${pct}%`, background: levelColor[level] }} />
          </div>
        )}
        <p className={own.note}>{note}</p>
      </div>
    </section>
  )
}

export function UsageTab() {
  const { t } = useT()
  const quota = useQuota()
  if (!quota.data) return <Skeleton height={220} />
  const { used, total } = quota.data
  const bwUsed = quota.data.bandwidth_used_month ?? 0
  const bwTotal = quota.data.bandwidth_quota_month ?? 0
  const period = quota.data.bandwidth_period

  return (
    <div className={own.stack}>
      <MeterCard
        kicker={t('settings.usageKicker')}
        used={used}
        total={total}
        note={
          <>
            {t('settings.usageNoteBefore')}
            <Link to="/trash" className={own.link}>
              {t('settings.usageNoteLink')}
            </Link>
            {t('settings.usageNoteAfter')}
          </>
        }
      />
      <MeterCard
        kicker={t('settings.bandwidthKicker')}
        used={bwUsed}
        total={bwTotal}
        period={period ? t('settings.bandwidthPeriod', { period }) : undefined}
        note={
          bwTotal > 0 ? t('settings.bandwidthNote') : t('settings.bandwidthUnlimitedNote')
        }
      />
    </div>
  )
}
