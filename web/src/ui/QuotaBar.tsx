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

/** 导航栏迷你容量条：标签 + 3px 进度 + 「已用 / 总量」。total≤0 表示不限，只显示已用。 */
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
  // 「2.14 / 10 GB」——两值同单位时只保留一次单位
  const usedText = formatBytes(used)
  const totalText = total > 0 ? formatBytes(total) : t('ui.quotaUnlimited')
  const [uNum, uUnit] = usedText.split(' ')
  const tUnit = total > 0 ? totalText.split(' ')[1] : ''
  const label =
    total > 0
      ? uUnit === tUnit
        ? `${uNum} / ${totalText}`
        : `${usedText} / ${totalText}`
      : usedText
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
