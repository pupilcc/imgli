import { Link } from 'react-router'
import { useConfig } from '../../api/hooks'
import { useT } from '../../i18n'
import { pickLocale } from '../../lib/locale'
import { EmptyState } from '../../ui/EmptyState'

export function AboutPage() {
  const { t, lang } = useT()
  const { data: cfg } = useConfig()
  if (!cfg?.about_enabled) {
    return (
      <div className="mx-auto max-w-[40rem] px-5 pt-8 pb-16">
        <EmptyState title={t('about.disabled')} desc={t('about.disabledDesc')}>
          <Link to="/">{t('common.backHome')}</Link>
        </EmptyState>
      </div>
    )
  }
  const body = pickLocale(cfg.about_body, lang).trim()
  return (
    <div className="mx-auto max-w-[40rem] px-5 pt-8 pb-16">
      <article className="rounded-lg border border-border bg-surface px-5 py-6">
        <h1 className="m-0 mb-4 text-[1.35rem] font-bold">{t('about.title')}</h1>
        {body ? (
          <pre className="m-0 whitespace-pre-wrap font-inherit text-[0.95rem] leading-[1.65] text-ink">{body}</pre>
        ) : (
          <p className="text-muted">{t('about.empty')}</p>
        )}
        <p className="mt-6 mb-0 text-[0.85rem]">
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
