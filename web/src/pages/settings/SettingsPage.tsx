import { Link, useNavigate, useParams } from 'react-router'
import { useSession } from '../../api/hooks'
import { useT } from '../../i18n'
import { PageHeader } from '../../shell/PageHeader'
import { PreferencesTab } from './PreferencesTab'
import { ProfileTab } from './ProfileTab'
import { TokensTab } from './TokensTab'
import { UsageTab } from './UsageTab'
import styles from './SettingsPage.module.css'

const TAB_KEYS = ['profile', 'preferences', 'tokens', 'usage'] as const

type TabKey = (typeof TAB_KEYS)[number]

export function SettingsPage() {
  const { t } = useT()
  const { tab: tabParam } = useParams()
  const navigate = useNavigate()
  const { data: me } = useSession()
  const tab: TabKey = TAB_KEYS.some((k) => k === tabParam) ? (tabParam as TabKey) : 'profile'

  const tabs: { key: TabKey; label: string }[] = [
    { key: 'profile', label: t('settings.tabProfile') },
    { key: 'preferences', label: t('settings.tabPreferences') },
    { key: 'tokens', label: t('settings.tabTokens') },
    { key: 'usage', label: t('settings.tabUsage') },
  ]

  return (
    <div className={styles.page}>
      <PageHeader kicker="SETTINGS" title={t('settings.title')} />
      {me?.is_admin && (
        <Link to="/admin" className={styles.adminEntry}>
          {t('settings.adminEntry')}
        </Link>
      )}
      <div className={styles.tabs}>
        {tabs.map((item) => (
          <button
            key={item.key}
            type="button"
            className={[styles.tab, tab === item.key && styles.tabActive].filter(Boolean).join(' ')}
            onClick={() => navigate(`/settings/${item.key}`, { replace: true })}
          >
            {item.label}
          </button>
        ))}
      </div>
      {tab === 'profile' && <ProfileTab />}
      {tab === 'preferences' && <PreferencesTab />}
      {tab === 'tokens' && <TokensTab />}
      {tab === 'usage' && <UsageTab />}
    </div>
  )
}
