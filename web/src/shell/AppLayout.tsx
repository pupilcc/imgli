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
    <>
      <Nav user={user} />
      {quota.data && <QuotaAlertBar used={quota.data.used} total={quota.data.total} />}
      <main className={styles.main}>
        <Outlet />
      </main>
      <SiteFooter footer={config?.footer} />
      <TabBar />
    </>
  )
}
