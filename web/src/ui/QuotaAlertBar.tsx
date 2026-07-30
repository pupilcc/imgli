import { Link } from 'react-router'
import { useT } from '../i18n'
import { quotaLevel } from './QuotaBar'
import styles from './QuotaAlertBar.module.css'

/** 配额通栏警告：≥80% warn、≥100% err 并由上传页禁用入口（③b 消费 level）。 */
export function QuotaAlertBar({
  used,
  total,
  upgradeUrl,
}: {
  used: number
  total: number
  /** Optional operator-configured upgrade / self-host URL from public config. */
  upgradeUrl?: string | null
}) {
  const { t } = useT()
  const level = quotaLevel(used, total)
  if (level === 'ok') return null
  const pct = Math.min(100, Math.round((used / total) * 100))
  const upgradeURL = (upgradeUrl || '').trim()
  return (
    <div className={`${styles.bar} ${level === 'full' ? styles.full : styles.warn}`}>
      {level === 'full' ? t('ui.quotaFull') : t('ui.quotaWarn', { pct })}
      <Link to="/settings" className={styles.link}>
        {t('ui.manageQuota')}
      </Link>
      {(level === 'full' || pct >= 80) && upgradeURL && (
        <a className={styles.link} href={upgradeURL} rel="noopener noreferrer">
          {t('ui.upgradeCta')}
        </a>
      )}
    </div>
  )
}
