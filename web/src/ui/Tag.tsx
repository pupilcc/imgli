import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

interface Props {
  variant?: 'ok' | 'muted' | 'warn' | 'err' | 'inverse'
  children: ReactNode
  className?: string
}

const variants: Record<NonNullable<Props['variant']>, string> = {
  ok: 'text-ok border-ok',
  muted: 'text-muted border-border',
  warn: 'text-warn border-warn',
  err: 'text-err border-err',
  inverse: 'bg-btn text-btn-text border-btn',
}

export function Tag({ variant = 'muted', children, className }: Props) {
  return (
    <span
      className={cn(
        'whitespace-nowrap rounded-sm border px-[7px] py-0.5 font-mono text-[9.5px] tracking-[0.08em]',
        variants[variant],
        className,
      )}
    >
      {children}
    </span>
  )
}
