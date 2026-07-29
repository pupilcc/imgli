import { useConfig, useSession } from '../api/hooks'
import { Skeleton } from '../ui/Skeleton'
import { AppLayout } from './AppLayout'
import { GuestLayout } from './GuestLayout'

/** `/`（上传页）壳：登录→完整布局；未登录→游客壳（始终可看上传页）。
 * 游客上传关时仍停留首页，由 UploadPage 提示登录/注册，避免直接甩到登录页。
 * 安全边界在后端（upload.Save 对游客开关做权威判定）。 */
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
  return <GuestLayout />
}
