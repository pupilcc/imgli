import { useEffect, useState, type ReactNode } from 'react'

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
}: {
  title: string
  armedTitle: string
  onConfirm(): void
  className?: string
  armedClassName?: string
  children?: ReactNode
  /** Visible label while armed; defaults to children (prefer a short confirm word for icon buttons). */
  armedChildren?: ReactNode
  stopPropagation?: boolean
}) {
  const [armed, setArmed] = useState(false)
  useEffect(() => {
    if (!armed) return
    const t = setTimeout(() => setArmed(false), 2500)
    return () => clearTimeout(t)
  }, [armed])

  return (
    <button
      type="button"
      title={armed ? armedTitle : title}
      aria-label={armed ? armedTitle : title}
      className={[className, armed && armedClassName].filter(Boolean).join(' ') || undefined}
      onClick={(e) => {
        if (stopPropagation) e.stopPropagation()
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
