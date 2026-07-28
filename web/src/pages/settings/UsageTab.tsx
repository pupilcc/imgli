import { Link } from 'react-router'
import { useQuota } from '../../api/hooks'
import { useT } from '../../i18n'
import { formatBytes } from '../../lib/format'
import { quotaLevel } from '../../ui/QuotaBar'
import { Skeleton } from '../../ui/Skeleton'
import styles from './SettingsPage.module.css'
import own from './UsageTab.module.css'

const levelColor = { ok: 'var(--text)', warn: 'var(--warn)', full: 'var(--err)' } as const

export function UsageTab() {
  const { t } = useT()
  const quota = useQuota()
  if (!quota.data) return <Skeleton height={120} />
  const { used, total } = quota.data
  const pct = total > 0 ? Math.min(100, (used / total) * 100) : 0
  const level = quotaLevel(used, total)
  return (
    <div>
      <div className={styles.kicker}>{t('settings.usageKicker')}</div>
      <div className={styles.card}>
        <div className={own.bigRow}>
          <div className={own.big}>
            {formatBytes(used)} <span className={own.bigSub}>/ {formatBytes(total)}</span>
          </div>
        </div>
        <div className={own.track}>
          <div className={own.fill} style={{ width: `${pct}%`, background: levelColor[level] }} />
        </div>
        <p className={own.note}>
          {t('settings.usageNoteBefore')}
          <Link to="/trash" className={own.link}>{t('settings.usageNoteLink')}</Link>
          {t('settings.usageNoteAfter')}
        </p>
      </div>
    </div>
  )
}
