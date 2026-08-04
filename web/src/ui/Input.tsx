import { useId, type InputHTMLAttributes, type ReactNode } from 'react'
import { cn } from '../lib/cn'

type Props = InputHTMLAttributes<HTMLInputElement> & {
  label?: ReactNode
  extra?: ReactNode
}

export function Input({ label, extra, id, className, ...rest }: Props) {
  const autoId = useId()
  const inputId = id ?? autoId
  return (
    <div className="flex flex-col gap-[7px]">
      {(label || extra) && (
        <div className="flex items-baseline justify-between">
          {label && (
            <label className="text-xs font-semibold text-muted" htmlFor={inputId}>
              {label}
            </label>
          )}
          {extra}
        </div>
      )}
      <input
        id={inputId}
        className={cn(
          'rounded-sm border border-border bg-surface px-3 py-2.5 font-inherit text-[13.5px] text-ink outline-none',
          'focus:border-muted',
          className,
        )}
        {...rest}
      />
    </div>
  )
}
