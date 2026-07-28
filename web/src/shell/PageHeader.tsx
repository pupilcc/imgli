import type { ReactNode } from 'react'
import styles from './PageHeader.module.css'

interface Props {
  kicker: string
  title: string
  extra?: ReactNode
}

export function PageHeader({ kicker, title, extra }: Props) {
  return (
    <div className={styles.head}>
      <div>
        <div className={styles.kicker}>{kicker}</div>
        <h1 className={styles.title}>{title}</h1>
      </div>
      {extra}
    </div>
  )
}
