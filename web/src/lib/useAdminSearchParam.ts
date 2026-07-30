import { useCallback } from 'react'
import { useSearchParams } from 'react-router'

/**
 * Admin list URL filter helper: set/delete a query key; non-page keys reset page.
 * Matches the setParam pattern used across Users / Logs / Invites / ImagesAdmin.
 */
export function useAdminSearchParam() {
  const [params, setParams] = useSearchParams()

  const setParam = useCallback(
    (key: string, value: string) => {
      setParams((p) => {
        const n = new URLSearchParams(p)
        if (value) n.set(key, value)
        else n.delete(key)
        if (key !== 'page') n.delete('page')
        return n
      })
    },
    [setParams],
  )

  return { params, setParams, setParam } as const
}
