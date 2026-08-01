import type { ReactNode } from 'react'
import styles from './StepGuide.module.css'

export type StepGuideProps = {
  kicker: string
  steps: string[]
  /** Optional footer actions (e.g. primary + dismiss). */
  actions?: ReactNode
  className?: string
  'data-testid'?: string
  id?: string
}

/** Compact 1–N step callout matching settings/admin surface language. */
export function StepGuide({
  kicker,
  steps,
  actions,
  className,
  id,
  'data-testid': testId,
}: StepGuideProps) {
  return (
    <aside
      id={id}
      className={[styles.card, className].filter(Boolean).join(' ')}
      role="note"
      data-testid={testId}
    >
      <div className={styles.kicker}>{kicker}</div>
      <ol className={styles.steps}>
        {steps.map((text, i) => (
          <li key={i} className={styles.step}>
            <span className={styles.num} aria-hidden>
              {String(i + 1).padStart(2, '0')}
            </span>
            <span className={styles.body}>{text}</span>
          </li>
        ))}
      </ol>
      {actions ? <div className={styles.actions}>{actions}</div> : null}
    </aside>
  )
}
