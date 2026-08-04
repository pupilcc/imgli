import { Link } from 'react-router'
import { useT } from '../i18n'
import { cn } from '../lib/cn'
import { quotaLevel } from './QuotaBar'

/** 配额通栏警告：≥80% warn、≥100% err 并由上传页禁用入口（③b 消费 level）。 */
export function QuotaAlertBar({
  used,
  total,
  upgradeUrl,
}: {
  used: number
  total: number
  upgradeUrl?: string | null
}) {
  const { t } = useT()
  const level = quotaLevel(used, total)
  if (level === 'ok') return null
  const pct = Math.min(100, Math.round((used / total) * 100))
  const upgradeURL = (upgradeUrl || '').trim()
  return (
    <div
      className={cn(
        'flex animate-[fadeIn_0.2s] items-center justify-center gap-2.5 border-b border-border bg-surface px-4 py-2 text-sm-plus font-semibold',
        level === 'full' ? 'text-err' : 'text-warn',
      )}
    >
      {level === 'full' ? t('ui.quotaFull') : t('ui.quotaWarn', { pct })}
      <Link to="/settings" className="font-bold text-inherit underline hover:text-inherit">
        {t('ui.manageQuota')}
      </Link>
      {(level === 'full' || pct >= 80) && upgradeURL && (
        <a className="font-bold text-inherit underline hover:text-inherit" href={upgradeURL} rel="noopener noreferrer">
          {t('ui.upgradeCta')}
        </a>
      )}
    </div>
  )
}
