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

/** 导航栏 132px 迷你容量条：STORAGE 标签 + 3px 进度 + 「已用 / 总量」。 */
export function QuotaBar({ used, total }: { used: number; total: number }) {
  const { t } = useT()
  const level = quotaLevel(used, total)
  const pct = total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0
  // 「2.14 / 10 GB」——两值同单位时只保留一次单位
  const usedText = formatBytes(used)
  const totalText = formatBytes(total)
  const [uNum, uUnit] = usedText.split(' ')
  const label = uUnit === totalText.split(' ')[1] ? `${uNum} / ${totalText}` : `${usedText} / ${totalText}`
  return (
    <Link to="/settings" title={t('ui.quotaTitle')} className={styles.wrap}>
      <div className={styles.labels}>
        <span>STORAGE</span>
        <span style={{ color: levelColor[level] }}>{label}</span>
      </div>
      <div className={styles.track}>
        <div className={styles.fill} style={{ width: `${pct}%`, background: levelColor[level] }} />
      </div>
    </Link>
  )
}
