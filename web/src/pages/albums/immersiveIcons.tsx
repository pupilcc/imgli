import type { ReactNode } from 'react'
import { cn } from '../../lib/cn'

/** 轻量内联图标（无依赖，无背景） */
export function ImmersiveIcon({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={cn('block size-[22px]', className)}
      aria-hidden
    >
      {children}
    </svg>
  )
}

export const IcoClose = () => (
  <ImmersiveIcon>
    <path d="M6 6l12 12M18 6L6 18" />
  </ImmersiveIcon>
)
export const IcoPrev = () => (
  <ImmersiveIcon>
    <path d="M15 6l-6 6 6 6" />
  </ImmersiveIcon>
)
export const IcoNext = () => (
  <ImmersiveIcon>
    <path d="M9 6l6 6-6 6" />
  </ImmersiveIcon>
)
export const IcoZoomIn = () => (
  <ImmersiveIcon>
    <circle cx="11" cy="11" r="6" />
    <path d="M16.5 16.5L21 21M11 8v6M8 11h6" />
  </ImmersiveIcon>
)
export const IcoZoomOut = () => (
  <ImmersiveIcon>
    <circle cx="11" cy="11" r="6" />
    <path d="M16.5 16.5L21 21M8 11h6" />
  </ImmersiveIcon>
)
export const IcoLink = () => (
  <ImmersiveIcon>
    <path d="M10 13a5 5 0 0 0 7.07 0l1.41-1.41a5 5 0 0 0-7.07-7.07L10 5.93" />
    <path d="M14 11a5 5 0 0 0-7.07 0L5.52 12.41a5 5 0 0 0 7.07 7.07L14 18.07" />
  </ImmersiveIcon>
)
export const IcoShare = () => (
  <ImmersiveIcon>
    <circle cx="18" cy="5" r="2.5" />
    <circle cx="6" cy="12" r="2.5" />
    <circle cx="18" cy="19" r="2.5" />
    <path d="M8.3 13.2l7.4 4.3M15.7 6.5l-7.4 4.3" />
  </ImmersiveIcon>
)
export const IcoExternal = () => (
  <ImmersiveIcon>
    <path d="M14 4h6v6M20 4l-9 9M10 6H6a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-4" />
  </ImmersiveIcon>
)

/** 无底透明圆形图标钮 */
export function IconBtn({
  label,
  onClick,
  disabled,
  className,
  children,
}: {
  label: string
  onClick?: () => void
  disabled?: boolean
  className?: string
  children: ReactNode
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        'flex size-11 cursor-pointer items-center justify-center rounded-full border-0 bg-transparent p-0 text-white/90',
        'transition-[color,transform,opacity] duration-150 hover:scale-105 hover:text-white',
        'disabled:pointer-events-none disabled:opacity-25',
        'focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white/50',
        className,
      )}
    >
      {children}
    </button>
  )
}
