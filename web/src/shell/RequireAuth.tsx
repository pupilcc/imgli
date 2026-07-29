import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router'
import { useSession } from '../api/hooks'
import { Skeleton } from '../ui/Skeleton'

export function RequireAuth({ children }: { children: ReactNode }) {
  const { data, isLoading } = useSession()
  const loc = useLocation()
  if (isLoading) {
    return (
      <div style={{ padding: 24, display: 'flex', flexDirection: 'column', gap: 16 }}>
        <Skeleton height={56} />
        <Skeleton height={220} />
      </div>
    )
  }
  if (!data) {
    const next = encodeURIComponent(`${loc.pathname}${loc.search}`)
    return <Navigate to={`/login?next=${next}`} replace />
  }
  return children
}
