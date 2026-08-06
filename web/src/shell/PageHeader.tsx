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
        // 实色底 + 底部阴影，避免列表滚过时「顶穿 / 透出」；标题与过滤区一并钉住
        'sticky top-0 z-[6] isolate mb-6 flex flex-wrap items-end justify-between gap-4 border-b border-border bg-bg py-3 pb-[18px] shadow-[0_10px_18px_-14px_rgba(0,0,0,0.28)]',
        className,
      )}
    >
      <div className="relative z-[1] min-w-0">
        <div className="mb-2 font-mono text-[11px] tracking-[0.14em] text-muted uppercase">{kicker}</div>
        <h1 className="m-0 text-[26px] font-bold tracking-[-0.015em] text-ink">{title}</h1>
      </div>
      {extra ? <div className="relative z-[1] min-w-0">{extra}</div> : null}
    </div>
  )
}
