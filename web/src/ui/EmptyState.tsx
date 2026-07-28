import type { ReactNode } from 'react'
import styles from './EmptyState.module.css'

interface Props {
  badge?: string
  title: string
  desc?: string
  children?: ReactNode
}

export function EmptyState({ badge = 'EMPTY', title, desc, children }: Props) {
  return (
    <div className={styles.wrap}>
      <div className={`stripe ${styles.thumb}`}>
        <span className={styles.badge}>{badge}</span>
      </div>
      <div className={styles.title}>{title}</div>
      {desc && <div className={styles.desc}>{desc}</div>}
      {children && <div className={styles.action}>{children}</div>}
    </div>
  )
}
