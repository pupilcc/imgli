import { useEffect, useState, type CSSProperties, type ReactNode } from 'react'
import { useT } from '../i18n'
import { Button } from './Button'

interface Props {
  label: ReactNode
  confirmLabel?: ReactNode
  onConfirm(): void
  timeoutMs?: number
  disabled?: boolean
}

// 轻量破坏操作的内联二次确认：一击待确认（err 边框），2.5s 未确认自动还原。
export function InlineConfirm({ label, confirmLabel, onConfirm, timeoutMs = 2500, disabled }: Props) {
  const { t } = useT()
  const [armed, setArmed] = useState(false)
  useEffect(() => {
    if (!armed) return
    const timer = setTimeout(() => setArmed(false), timeoutMs)
    return () => clearTimeout(timer)
  }, [armed, timeoutMs])

  const resolvedConfirm = confirmLabel ?? t('ui.confirmDelete')
  const idleStyle: CSSProperties = { color: 'var(--err)' }
  if (!armed) {
    return (
      <Button variant="secondary" style={idleStyle} disabled={disabled} onClick={() => setArmed(true)}>
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
