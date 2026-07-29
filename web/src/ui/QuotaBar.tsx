import { Link } from 'react-router'
import { useT } from '../i18n'
import { formatBytes } from '../lib/format'
import styles from './QuotaBar.module.css'

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
    <Link to={to} title={title} className={styles.wrap}>
      <div className={styles.labels}>
        <span>{tag}</span>
        <span style={{ color: levelColor[level] }}>{label}</span>
      </div>
      <div className={styles.track}>
        <div
          className={styles.fill}
          style={{ width: total > 0 ? `${pct}%` : '0%', background: levelColor[level] }}
        />
      </div>
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
  const title = showBw
    ? `${t('ui.quotaTitle')}: ${formatPair(storage.used, storage.total)} · ${t('ui.bandwidthTitle')}: ${formatPair(bandwidth!.used, bandwidth!.total)}`
    : `${t('ui.quotaTitle')}: ${formatPair(storage.used, storage.total)}`

  return (
    <Link to={to} title={title} className={styles.cluster} data-testid="nav-quota-cluster">
      <div className={styles.clusterRow}>
        <span className={styles.clusterTag}>{t('ui.navStorageShort')}</span>
        <span className={styles.clusterVal} style={{ color: levelColor[sLevel] }}>
          {formatPair(storage.used, storage.total)}
        </span>
      </div>
      <div className={styles.track}>
        <div
          className={styles.fill}
          style={{ width: storage.total > 0 ? `${sPct}%` : '0%', background: levelColor[sLevel] }}
        />
      </div>
      {showBw && (
        <>
          <div className={styles.clusterRow}>
            <span className={styles.clusterTag}>{t('ui.navBandwidthShort')}</span>
            <span className={styles.clusterVal} style={{ color: levelColor[bLevel] }}>
              {formatPair(bandwidth.used, bandwidth.total)}
            </span>
          </div>
          <div className={styles.track}>
            <div className={styles.fill} style={{ width: `${bPct}%`, background: levelColor[bLevel] }} />
          </div>
        </>
      )}
    </Link>
  )
}
