import { Link } from 'react-router'
import { useT } from '../i18n'
import { quotaLevel } from './QuotaBar'
import styles from './QuotaAlertBar.module.css'

/** 配额通栏警告：≥80% warn、≥100% err 并由上传页禁用入口（③b 消费 level）。 */
export function QuotaAlertBar({ used, total }: { used: number; total: number }) {
  const { t } = useT()
  const level = quotaLevel(used, total)
  if (level === 'ok') return null
  const pct = Math.min(100, Math.round((used / total) * 100))
  return (
    <div className={`${styles.bar} ${level === 'full' ? styles.full : styles.warn}`}>
      {level === 'full' ? t('ui.quotaFull') : t('ui.quotaWarn', { pct })}
      <Link to="/settings" className={styles.link}>
        {t('ui.manageQuota')}
      </Link>
    </div>
  )
}
