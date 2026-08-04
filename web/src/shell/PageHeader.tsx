import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

interface Props {
  kicker: string
  title: string
  extra?: ReactNode
  className?: string
}

export function PageHeader({ kicker, title, extra, className }: Props) {
  return (
    <div
      className={cn(
        'sticky top-0 z-[6] -mt-2 mb-6 flex flex-wrap items-end justify-between gap-4 border-b border-border bg-bg py-3 pb-[18px] shadow-[0_1px_0_var(--border)]',
        className,
      )}
    >
      <div>
        <div className="mb-2 font-mono text-[11px] tracking-[0.14em] text-muted uppercase">{kicker}</div>
        <h1 className="m-0 text-[26px] font-bold tracking-[-0.015em]">{title}</h1>
      </div>
      {extra}
    </div>
  )
}
