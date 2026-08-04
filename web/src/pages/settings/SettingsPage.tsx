import { Link, useNavigate, useParams } from 'react-router'
import { useSession } from '../../api/hooks'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { PageHeader } from '../../shell/PageHeader'
import { PreferencesTab } from './PreferencesTab'
import { ProfileTab } from './ProfileTab'
import { TokensTab } from './TokensTab'
import { UsageTab } from './UsageTab'

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
    <div className="mx-auto max-w-[880px] pt-11">
      <PageHeader kicker="SETTINGS" title={t('settings.title')} />
      {me?.is_admin && (
        <Link
          to="/admin"
          className="mb-4 flex items-center justify-between rounded-sm border border-border bg-surface px-3.5 py-3 text-[13px] font-semibold hover:bg-soft"
        >
          {t('settings.adminEntry')}
        </Link>
      )}
      <div className="mb-[26px] flex gap-[22px] border-b border-border">
        {tabs.map((item) => (
          <button
            key={item.key}
            type="button"
            className={cn(
              'cursor-pointer border-0 bg-transparent px-0.5 pb-[11px] text-[13px] font-semibold text-muted hover:text-ink',
              tab === item.key && 'text-ink shadow-[inset_0_-2px_0_var(--text)]',
            )}
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
