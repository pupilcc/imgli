import type { ButtonHTMLAttributes } from 'react'
import { cn } from '../lib/cn'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
}

const variants: Record<NonNullable<Props['variant']>, string> = {
  primary: 'border-0 bg-btn text-btn-text font-bold hover:enabled:opacity-85',
  secondary: 'border border-border bg-surface text-ink font-semibold hover:enabled:bg-soft',
  danger: 'border border-err bg-surface text-err font-bold hover:enabled:bg-soft',
  ghost: 'border-0 bg-transparent text-muted font-semibold underline px-1 py-2 hover:enabled:text-ink',
}

export function Button({ variant = 'secondary', className, type = 'button', ...rest }: Props) {
  return (
    <button
      type={type}
      className={cn(
        'cursor-pointer rounded-sm text-sm-plus px-[18px] py-[9px] transition-[opacity,background,color] duration-150',
        'disabled:cursor-not-allowed disabled:opacity-50',
        variants[variant],
        className,
      )}
      {...rest}
    />
  )
}
