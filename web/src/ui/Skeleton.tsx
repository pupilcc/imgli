import type { CSSProperties } from 'react'
import styles from './Skeleton.module.css'

interface Props {
  width?: number | string
  height?: number | string
  style?: CSSProperties
}

export function Skeleton({ width = '100%', height = 12, style }: Props) {
  return <div className={styles.block} style={{ width, height, ...style }} />
}
