import { Route, Routes } from 'react-router'
import { useT } from '../../i18n'
import { EmptyState } from '../../ui/EmptyState'
import { DashboardPage } from './dashboard/DashboardPage'
import { GroupsPage } from './groups/GroupsPage'
import { ImagesAdminPage } from './images/ImagesAdminPage'
import { InvitesPage } from './invites/InvitesPage'
import { LogsPage } from './logs/LogsPage'
import { PoliciesPage } from './policies/PoliciesPage'
import { ReviewPage } from './review/ReviewPage'
import { SettingsPage } from './settings/SettingsPage'
import { AdminLayout } from './shell/AdminLayout'
import { SystemPage } from './system/SystemPage'
import { UsersPage } from './users/UsersPage'

function AdminNotFound() {
  const { t } = useT()
  return <EmptyState badge="404" title={t('adminB.pageNotFound')} desc={t('adminB.pageNotFoundDesc')} />
}

export default function AdminApp() {
  return (
    <Routes>
      <Route element={<AdminLayout />}>
        <Route index element={<DashboardPage />} />
        <Route path="users" element={<UsersPage />} />
        <Route path="images" element={<ImagesAdminPage />} />
        <Route path="review" element={<ReviewPage />} />
        <Route path="groups" element={<GroupsPage />} />
        <Route path="invites" element={<InvitesPage />} />
        <Route path="policies" element={<PoliciesPage />} />
        <Route path="system" element={<SystemPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="logs" element={<LogsPage />} />
        <Route path="*" element={<AdminNotFound />} />
      </Route>
    </Routes>
  )
}
