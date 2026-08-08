import type { CSSProperties, ReactNode } from 'react'
import { cn } from '../lib/cn'

interface Props<T extends string> {
  options: { value: T; label: ReactNode }[]
  value: T
  onChange(v: T): void
  mono?: boolean
  /** Tighter padding; wraps when the bar is narrow (detail pane, mobile). */
  compact?: boolean
}

/** 选中态直接绑 CSS 变量，避免 text-btn-text / text-muted 等工具类冲突导致「要 hover 才看见字」。 */
const activeStyle: CSSProperties = {
  backgroundColor: 'var(--btn)',
  color: 'var(--btnText)',
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
            data-segmented-active={active ? '1' : undefined}
            style={active ? activeStyle : undefined}
            className={cn(
              'min-w-0 flex-1 cursor-pointer border-0 px-4 py-2 text-xs font-semibold transition-[background-color,opacity] duration-150',
              'overflow-hidden text-ellipsis whitespace-nowrap',
              i > 0 && 'border-l border-border',
              // 未选中：面板色 + 次要字色；选中：data-segmented-active + inline style 强制 --btn/--btnText
              !active && 'bg-surface text-muted hover:bg-soft',
              active && 'hover:opacity-90',
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
