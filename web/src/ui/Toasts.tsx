import { useGlobal } from '../store'
import styles from './Toasts.module.css'

export function Toasts() {
  const toasts = useGlobal((s) => s.toasts)
  if (toasts.length === 0) return null
  return (
    <div className={styles.wrap} aria-live="polite">
      {toasts.map((t) => (
        <div key={t.id} className={styles.toast}>
          {t.message}
        </div>
      ))}
    </div>
  )
}
