import type { ReactNode } from 'react'
import { Skeleton } from '../../../ui/Skeleton'
import { AdminError } from './AdminError'

/** Standard admin query shell: error → retry, loading → skeleton, else children(data). */
export function AdminQueryGate<T>({
  query,
  height = 220,
  children,
}: {
  query: {
    isError: boolean
    data: T | null | undefined
    refetch: () => unknown
  }
  height?: number
  children: (data: T) => ReactNode
}) {
  if (query.isError) {
    return <AdminError onRetry={() => query.refetch()} />
  }
  // loading / placeholder：undefined 与 null 均展示骨架（勿把 null 传给 children）
  if (query.data == null) {
    return <Skeleton height={height} />
  }
  return <>{children(query.data)}</>
}
