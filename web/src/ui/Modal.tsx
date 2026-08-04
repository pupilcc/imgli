import { useEffect, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { cn } from '../lib/cn'

interface Props {
  open: boolean
  onClose(): void
  width?: number
  children: ReactNode
  className?: string
}

export function Modal({ open, onClose, width = 420, children, className }: Props) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null
  return createPortal(
    <div
      className="fixed inset-0 z-[90] flex animate-[fadeIn_0.15s] items-center justify-center bg-black/50"
      onClick={onClose}
    >
      <div
        role="dialog"
        className={cn(
          'box-border max-w-[calc(100vw-32px)] animate-[rise_0.28s] rounded border border-border bg-surface p-[18px] shadow-[0_12px_32px_rgba(0,0,0,0.12)]',
          className,
        )}
        style={{ width }}
        onClick={(e) => e.stopPropagation()}
      >
        {children}
      </div>
    </div>,
    document.body,
  )
}
