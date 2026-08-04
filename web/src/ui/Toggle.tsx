import { cn } from '../lib/cn'

interface Props {
  checked: boolean
  onChange(v: boolean): void
  disabled?: boolean
  'aria-label'?: string
}

export function Toggle({ checked, onChange, disabled, 'aria-label': ariaLabel }: Props) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      className={cn(
        'relative h-[18px] w-[34px] cursor-pointer rounded-sm border border-border bg-soft p-0 transition-colors duration-150',
        'disabled:cursor-not-allowed disabled:opacity-50',
        checked && 'border-btn bg-btn',
      )}
      onClick={() => onChange(!checked)}
    >
      <span
        className={cn(
          'absolute top-0.5 left-0.5 h-3 w-3 rounded-sm bg-muted transition-[left,background] duration-150',
          checked && 'left-[18px] bg-btn-text',
        )}
      />
    </button>
  )
}
