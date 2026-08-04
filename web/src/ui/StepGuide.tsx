import type { ReactNode } from 'react'
import { cn } from '../lib/cn'

export type StepGuideProps = {
  kicker: string
  steps: string[]
  actions?: ReactNode
  className?: string
  'data-testid'?: string
  id?: string
}

/** Compact 1–N step callout matching settings/admin console language. */
export function StepGuide({
  kicker,
  steps,
  actions,
  className,
  id,
  'data-testid': testId,
}: StepGuideProps) {
  return (
    <aside
      id={id}
      className={cn('mb-4 rounded-sm border border-border bg-surface px-[18px] py-4 text-ink max-[560px]:p-3.5', className)}
      role="note"
      data-testid={testId}
    >
      <div className="mb-3 font-mono text-2xs tracking-[0.12em] text-muted">{kicker}</div>
      <ol className="m-0 flex list-none flex-col gap-2.5 p-0">
        {steps.map((text, i) => (
          <li key={i} className="grid grid-cols-[28px_1fr] items-start gap-3">
            <span
              className="flex h-7 w-7 flex-none items-center justify-center rounded-sm border border-border bg-soft font-mono text-2xs font-bold tracking-[0.06em] text-muted"
              aria-hidden
            >
              {String(i + 1).padStart(2, '0')}
            </span>
            <span className="pt-1 text-[13px] leading-normal text-ink">{text}</span>
          </li>
        ))}
      </ol>
      {actions ? <div className="mt-3.5 flex flex-wrap items-center gap-2 [&_a]:no-underline">{actions}</div> : null}
    </aside>
  )
}
