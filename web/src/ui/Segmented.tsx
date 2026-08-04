import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

interface Props<T extends string> {
  options: { value: T; label: ReactNode }[]
  value: T
  onChange(v: T): void
  mono?: boolean
  /** Tighter padding; wraps when the bar is narrow (detail pane, mobile). */
  compact?: boolean
}

export function Segmented<T extends string>({ options, value, onChange, mono, compact }: Props<T>) {
  return (
    <div
      className={cn(
        'flex min-w-0 flex-nowrap overflow-hidden rounded-sm border border-border',
        compact && 'flex-wrap overflow-visible max-[420px]:grid max-[420px]:grid-cols-[repeat(auto-fit,minmax(4.5rem,1fr))]',
      )}
    >
      {options.map((o, i) => {
        const active = o.value === value
        return (
          <button
            key={o.value}
            type="button"
            aria-pressed={active}
            className={cn(
              'min-w-0 flex-1 cursor-pointer border-0 bg-surface px-4 py-2 text-xs font-semibold text-muted transition-colors duration-150',
              'overflow-hidden text-ellipsis whitespace-nowrap hover:bg-soft',
              i > 0 && 'border-l border-border',
              active && 'bg-btn text-btn-text hover:bg-btn',
              mono && 'font-mono text-xs-plus',
              compact && 'flex-[1_1_auto] px-2 py-1.5 text-xs-plus',
              compact && mono && 'text-2xs',
              compact &&
                'max-[420px]:border-r max-[420px]:border-b max-[420px]:border-border max-[420px]:border-l-0',
            )}
            onClick={() => onChange(o.value)}
          >
            {o.label}
          </button>
        )
      })}
    </div>
  )
}
