import type { ReactNode } from 'react'
import styles from './Tag.module.css'

interface Props {
  variant?: 'ok' | 'muted' | 'warn' | 'err' | 'inverse'
  children: ReactNode
}

export function Tag({ variant = 'muted', children }: Props) {
  return <span className={[styles.tag, styles[variant]].join(' ')}>{children}</span>
}
