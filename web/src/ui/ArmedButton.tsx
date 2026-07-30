import { useEffect, useState, type CSSProperties, type ReactNode } from 'react'

/** Two-click confirm control (first click arms for 2.5s, second confirms). */
export function ArmedButton({
  title,
  armedTitle,
  onConfirm,
  className,
  armedClassName,
  children,
  stopPropagation = true,
}: {
  title: string
  armedTitle: string
  onConfirm(): void
  className?: string
  armedClassName?: string
  children?: ReactNode
  stopPropagation?: boolean
}) {
  const [armed, setArmed] = useState(false)
  useEffect(() => {
    if (!armed) return
    const t = setTimeout(() => setArmed(false), 2500)
    return () => clearTimeout(t)
  }, [armed])

  const style: CSSProperties | undefined = undefined
  return (
    <button
      type="button"
      title={armed ? armedTitle : title}
      className={[className, armed && armedClassName].filter(Boolean).join(' ') || undefined}
      style={style}
      onClick={(e) => {
        if (stopPropagation) e.stopPropagation()
        if (armed) {
          setArmed(false)
          onConfirm()
        } else setArmed(true)
      }}
    >
      {children}
    </button>
  )
}
