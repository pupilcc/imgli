import { Link } from 'react-router'
import { useT } from '../i18n'
import { formatBytes } from '../lib/format'

export type QuotaLevel = 'ok' | 'warn' | 'full'

export function quotaLevel(used: number, total: number): QuotaLevel {
  if (total <= 0) return 'ok'
  const pct = (used / total) * 100
  if (pct >= 100) return 'full'
  if (pct >= 80) return 'warn'
  return 'ok'
}

const levelColor: Record<QuotaLevel, string> = {
  ok: 'var(--text)',
  warn: 'var(--warn)',
  full: 'var(--err)',
}

export type QuotaBarKind = 'storage' | 'bandwidth'

function formatPair(used: number, total: number): string {
  const usedText = formatBytes(used)
  if (total <= 0) return usedText
  const totalText = formatBytes(total)
  const [uNum, uUnit] = usedText.split(' ')
  const tUnit = totalText.split(' ')[1]
  return uUnit === tUnit ? `${uNum} / ${totalText}` : `${usedText} / ${totalText}`
}

function Track({ pct, color }: { pct: number; color: string }) {
  return (
    <div className="h-[3px] overflow-hidden rounded-sm bg-soft">
      <div className="h-full transition-[width] duration-300" style={{ width: `${pct}%`, background: color }} />
    </div>
  )
}

/** 单条迷你进度（上传页并排等较宽场景）。 */
export function QuotaBar({
  used,
  total,
  kind = 'storage',
  to = '/settings',
}: {
  used: number
  total: number
  kind?: QuotaBarKind
  to?: string
}) {
  const { t } = useT()
  const level = quotaLevel(used, total)
  const pct = total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0
  const label =
    total > 0 ? formatPair(used, total) : `${formatBytes(used)} / ${t('ui.quotaUnlimited')}`
  const tag = kind === 'bandwidth' ? 'BANDWIDTH' : 'STORAGE'
  const title = kind === 'bandwidth' ? t('ui.bandwidthTitle') : t('ui.quotaTitle')
  return (
    <Link to={to} title={title} className="flex w-[148px] flex-none flex-col gap-1 max-[900px]:hidden">
      <div className="flex justify-between gap-1.5 font-mono text-[9.5px] tracking-[0.04em] text-muted">
        <span>{tag}</span>
        <span style={{ color: levelColor[level] }}>{label}</span>
      </div>
      <Track pct={total > 0 ? pct : 0} color={levelColor[level]} />
    </Link>
  )
}

export type MeterSlice = { used: number; total: number }

/**
 * 导航栏紧凑双条：叠在同一块 148px 宽区域内，减少横向拥挤。
 * bandwidth 为 null/undefined 或 total≤0 时只显示存储。
 */
export function NavQuotaCluster({
  storage,
  bandwidth,
  to = '/settings',
}: {
  storage: MeterSlice
  bandwidth?: MeterSlice | null
  to?: string
}) {
  const { t } = useT()
  const showBw = !!bandwidth && bandwidth.total > 0
  const sLevel = quotaLevel(storage.used, storage.total)
  const bLevel = showBw ? quotaLevel(bandwidth.used, bandwidth.total) : 'ok'
  const sPct = storage.total > 0 ? Math.min(100, Math.round((storage.used / storage.total) * 100)) : 0
  const bPct =
    showBw && bandwidth.total > 0
      ? Math.min(100, Math.round((bandwidth.used / bandwidth.total) * 100))
      : 0
  const trashHint = t('ui.quotaTitleHint')
  const title = showBw
    ? `${t('ui.quotaTitle')}: ${formatPair(storage.used, storage.total)} (${trashHint}) · ${t('ui.bandwidthTitle')}: ${formatPair(bandwidth!.used, bandwidth!.total)}`
    : `${t('ui.quotaTitle')}: ${formatPair(storage.used, storage.total)} (${trashHint})`

  return (
    <Link
      to={to}
      title={title}
      className="flex w-[148px] flex-none flex-col gap-0.5 py-1 text-inherit no-underline hover:text-inherit max-[900px]:hidden"
      data-testid="nav-quota-cluster"
    >
      <div className="flex items-baseline justify-between gap-1.5 font-mono text-[9.5px] leading-tight tracking-[0.03em]">
        <span className="flex-none text-muted">{t('ui.navStorageShort')}</span>
        <span className="min-w-0 overflow-hidden text-right text-ellipsis whitespace-nowrap" style={{ color: levelColor[sLevel] }}>
          {formatPair(storage.used, storage.total)}
        </span>
      </div>
      <Track pct={storage.total > 0 ? sPct : 0} color={levelColor[sLevel]} />
      {showBw && (
        <>
          <div className="flex items-baseline justify-between gap-1.5 font-mono text-[9.5px] leading-tight tracking-[0.03em]">
            <span className="flex-none text-muted">{t('ui.navBandwidthShort')}</span>
            <span className="min-w-0 overflow-hidden text-right text-ellipsis whitespace-nowrap" style={{ color: levelColor[bLevel] }}>
              {formatPair(bandwidth.used, bandwidth.total)}
            </span>
          </div>
          <Track pct={bPct} color={levelColor[bLevel]} />
        </>
      )}
    </Link>
  )
}
