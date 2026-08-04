import { useEffect, useState, type ReactNode } from 'react'
import { cn } from '../lib/cn'

/** Two-click confirm control (first click arms for 2.5s, second confirms). */
export function ArmedButton({
  title,
  armedTitle,
  onConfirm,
  className,
  armedClassName,
  children,
  armedChildren,
  stopPropagation = true,
  disabled,
}: {
  title: string
  armedTitle: string
  onConfirm(): void
  className?: string
  armedClassName?: string
  children?: ReactNode
  armedChildren?: ReactNode
  stopPropagation?: boolean
  disabled?: boolean
}) {
  const [armed, setArmed] = useState(false)
  useEffect(() => {
    if (!armed) return
    const t = setTimeout(() => setArmed(false), 2500)
    return () => clearTimeout(t)
  }, [armed])
  useEffect(() => {
    if (disabled) setArmed(false)
  }, [disabled])

  return (
    <button
      type="button"
      title={armed ? armedTitle : title}
      aria-label={armed ? armedTitle : title}
      disabled={disabled}
      className={cn(className, armed && armedClassName)}
      onClick={(e) => {
        if (stopPropagation) e.stopPropagation()
        if (disabled) return
        if (armed) {
          setArmed(false)
          onConfirm()
        } else setArmed(true)
      }}
    >
      {armed ? (armedChildren ?? children) : children}
    </button>
  )
}
