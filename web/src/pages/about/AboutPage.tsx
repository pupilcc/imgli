import { Link } from 'react-router'
import { useConfig } from '../../api/hooks'
import { useT } from '../../i18n'
import { pickLocale } from '../../lib/locale'
import { EmptyState } from '../../ui/EmptyState'
import styles from './AboutPage.module.css'

export function AboutPage() {
  const { t, lang } = useT()
  const { data: cfg } = useConfig()
  if (!cfg?.about_enabled) {
    return (
      <div className={styles.wrap}>
        <EmptyState title={t('about.disabled')} desc={t('about.disabledDesc')}>
          <Link to="/">{t('common.backHome')}</Link>
        </EmptyState>
      </div>
    )
  }
  const body = pickLocale(cfg.about_body, lang).trim()
  return (
    <div className={styles.wrap}>
      <article className={styles.card}>
        <h1 className={styles.title}>{t('about.title')}</h1>
        {body ? (
          <pre className={styles.body}>{body}</pre>
        ) : (
          <p className={styles.muted}>{t('about.empty')}</p>
        )}
        <p className={styles.meta}>
          <Link to="/">{t('common.backHome')}</Link>
          {cfg.source_url ? (
            <>
              {' · '}
              <a href={cfg.source_url} rel="noopener noreferrer">
                {t('common.sourceCode')}
              </a>
            </>
          ) : null}
        </p>
      </article>
    </div>
  )
}
