import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

interface Props {
  kicker: string
  title: string
  extra?: ReactNode
  className?: string
}

/**
 * Page title strip. Layout contract (do not break again):
 *
 * - Outer box width === sibling content (no -mx on the default front-end path).
 * - Horizontal/vertical padding is ON this box so title text is never flush to
 *   the surface edges (padL/padR must stay non-zero).
 * - Front-end: sticky under app nav (`top-14`); gutters come from main + this px.
 * - Admin: AdminLayout sets top-0 and -mx/px so the strip bleeds to the main
 *   column edges while keeping the same text inset as body content.
 */
export function PageHeader({ kicker, title, extra, className }: Props) {
  return (
    <div
      className={cn(
        'page-header sticky top-14 z-[6] isolate mb-6 flex flex-wrap items-end justify-between gap-4',
        // Internal padding only — keeps outer width equal to content below.
        'px-5 py-5 max-md:px-4',
        'border-b border-border bg-surface shadow-[0_10px_18px_-14px_rgba(0,0,0,0.28)]',
        className,
      )}
      data-testid="page-header"
    >
      <div className="relative z-[1] min-w-0">
        <div className="mb-2 font-mono text-[11px] font-semibold tracking-[0.14em] text-muted uppercase">
          {kicker}
        </div>
        <h1 className="m-0 text-[26px] font-bold tracking-[-0.015em] text-ink">{title}</h1>
      </div>
      {extra ? <div className="relative z-[1] min-w-0">{extra}</div> : null}
    </div>
  )
}
