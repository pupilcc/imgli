import { useId, type InputHTMLAttributes, type ReactNode } from 'react'
import styles from './Input.module.css'

type Props = InputHTMLAttributes<HTMLInputElement> & {
  label?: ReactNode
  extra?: ReactNode
}

export function Input({ label, extra, id, className, ...rest }: Props) {
  const autoId = useId()
  const inputId = id ?? autoId
  return (
    <div className={styles.field}>
      {(label || extra) && (
        <div className={styles.labelRow}>
          {label && (
            <label className={styles.label} htmlFor={inputId}>
              {label}
            </label>
          )}
          {extra}
        </div>
      )}
      <input id={inputId} className={[styles.input, className].filter(Boolean).join(' ')} {...rest} />
    </div>
  )
}
