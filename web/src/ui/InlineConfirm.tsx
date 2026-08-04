import { useEffect, useState, type ReactNode } from 'react'
import { useT } from '../i18n'
import { Button } from './Button'

interface Props {
  label: ReactNode
  confirmLabel?: ReactNode
  onConfirm(): void
  timeoutMs?: number
  disabled?: boolean
}

/** Inline two-step confirm using Button variants. */
export function InlineConfirm({ label, confirmLabel, onConfirm, timeoutMs = 2500, disabled }: Props) {
  const { t } = useT()
  const [armed, setArmed] = useState(false)
  useEffect(() => {
    if (!armed) return
    const timer = setTimeout(() => setArmed(false), timeoutMs)
    return () => clearTimeout(timer)
  }, [armed, timeoutMs])

  const resolvedConfirm = confirmLabel ?? t('ui.confirmDelete')
  if (!armed) {
    return (
      <Button variant="secondary" className="text-err" disabled={disabled} onClick={() => setArmed(true)}>
        {label}
      </Button>
    )
  }
  return (
    <Button
      variant="danger"
      disabled={disabled}
      onClick={() => {
        setArmed(false)
        onConfirm()
      }}
    >
      {resolvedConfirm}
    </Button>
  )
}
