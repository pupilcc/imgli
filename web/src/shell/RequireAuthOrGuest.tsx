import { Navigate } from 'react-router'
import { useConfig, useSession } from '../api/hooks'
import { Skeleton } from '../ui/Skeleton'
import { AppLayout } from './AppLayout'
import { GuestLayout } from './GuestLayout'

/** `/`（上传页）门禁：登录→完整布局；未登录且游客上传开→游客布局；否则跳登录。
 * 体验层；安全边界在后端（upload.Save 对游客开关做权威判定）。 */
export function RequireAuthOrGuest() {
  const { data: user, isLoading } = useSession()
  const config = useConfig()
  if (isLoading || (!user && config.isLoading)) {
    return (
      <div style={{ padding: 24, display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Skeleton height={56} />
        <Skeleton height={220} />
      </div>
    )
  }
  if (user) return <AppLayout />
  if (config.data?.guest_upload_enabled) return <GuestLayout />
  return <Navigate to="/login" replace />
}
