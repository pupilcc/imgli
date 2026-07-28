import type { ReactNode } from 'react'
import { Navigate } from 'react-router'
import { useSession } from '../api/hooks'
import { Skeleton } from '../ui/Skeleton'

/** admin 门禁(体验层;安全边界在后端 RequireAdmin 中间件)。 */
export function RequireAdmin({ children }: { children: ReactNode }) {
  const { data, isLoading } = useSession()
  if (isLoading) {
    return (
      <div style={{ padding: 24, display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Skeleton height={56} />
        <Skeleton height={220} />
      </div>
    )
  }
  if (!data) return <Navigate to="/login" replace />
  if (!data.is_admin) return <Navigate to="/" replace />
  return children
}
