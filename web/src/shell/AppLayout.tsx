import { Outlet } from 'react-router'
import { useConfig, useQuota, useSession } from '../api/hooks'
import { QuotaAlertBar } from '../ui/QuotaAlertBar'
import { SiteFooter } from '../ui/SiteSlots'
import { Nav } from './Nav'
import { TabBar } from './TabBar'
import styles from './AppLayout.module.css'

export function AppLayout() {
  const { data: user } = useSession()
  const { data: config } = useConfig()
  const quota = useQuota()
  if (!user) return null
  return (
    <div className={styles.shell}>
      <Nav user={user} />
      {quota.data && (
        <QuotaAlertBar
          used={quota.data.used}
          total={quota.data.total}
          upgradeUrl={config?.upgrade_url}
        />
      )}
      <main className={styles.main}>
        <Outlet />
      </main>
      <SiteFooter
        footer={config?.footer}
        siteName={config?.site_name}
        ossCredit={config?.oss_credit}
        sourceUrl={config?.source_url}
        aboutEnabled={!!config?.about_enabled}
      />
      <TabBar />
    </div>
  )
}
