import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { useQuota } from '../../api/hooks'
import { useT } from '../../i18n'
import { formatBytes } from '../../lib/format'
import { quotaLevel } from '../../ui/QuotaBar'
import { Skeleton } from '../../ui/Skeleton'

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
      <div className="mb-3 font-mono text-2xs tracking-[0.14em] text-muted">{kicker}</div>
      <div className="flex flex-col gap-3.5 rounded-sm border border-border bg-surface p-[18px]">
        <div className="flex items-baseline justify-between gap-3">
          <div className="text-[22px] font-extrabold tracking-[-0.01em]">
            {formatBytes(used)}{' '}
            <span className="text-[13px] font-semibold text-muted">
              / {unlimited ? t('ui.quotaUnlimited') : formatBytes(total)}
            </span>
          </div>
          {period ? (
            <span className="shrink-0 font-mono text-[11px] tracking-[0.04em] text-muted">{period}</span>
          ) : null}
        </div>
        {!unlimited && (
          <div className="my-0 flex h-2 overflow-hidden rounded-[2px] bg-soft">
            <div
              className="h-full transition-[width] duration-300"
              style={{ width: `${pct}%`, background: levelColor[level] }}
            />
          </div>
        )}
        <p className="m-0 text-xs leading-relaxed text-muted">{note}</p>
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
    <div className="flex flex-col gap-3.5">
      <MeterCard
        kicker={t('settings.usageKicker')}
        used={used}
        total={total}
        note={
          <>
            {t('settings.usageNoteBefore')}
            <Link to="/trash" className="text-muted underline hover:text-ink">
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
