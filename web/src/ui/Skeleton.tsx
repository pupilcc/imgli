import type { CSSProperties } from 'react'
import { cn } from '../lib/cn'

interface Props {
  width?: number | string
  height?: number | string
  style?: CSSProperties
  className?: string
}

export function Skeleton({ width = '100%', height = 12, style, className }: Props) {
  return (
    <div
      className={cn('animate-[pulse_1.4s_infinite] rounded-sm bg-soft', className)}
      style={{ width, height, ...style }}
    />
  )
}
