import { useState } from 'react'
import { useParams, Link } from 'react-router'
import { ApiError } from '../../api/client'
import { useShareImage, useUnlockShare } from '../../api/hooks'
import { useT } from '../../i18n'
import { copyText } from '../../lib/copy'
import { formatBytes } from '../../lib/format'
import { BrandLockup } from '../../ui/Brand'
import { Button } from '../../ui/Button'
import { Input } from '../../ui/Input'
import { LangToggle } from '../../ui/LangToggle'
import { useGlobal } from '../../store'
import styles from './SharePage.module.css'

/** Public share landing: preview + copy links for public/normal images. */
export function SharePage() {
  const { key = '' } = useParams()
  const { t, lang } = useT()
  const theme = useGlobal((s) => s.theme)
  const toggleTheme = useGlobal((s) => s.toggleTheme)
  const q = useShareImage(key)
  const unlock = useUnlockShare()
  const data = q.data
  const [pw, setPw] = useState('')

  const notFound = q.error instanceof ApiError && q.error.httpStatus === 404
  const needPw = !!(data?.password_required || (data?.has_access_password && !data?.links?.url))

  let expiryLabel = ''
  if (data?.expires_at) {
    const d = new Date(data.expires_at)
    expiryLabel = d.toLocaleString(lang === 'zh' ? 'zh-CN' : 'en-US')
  }

  return (
    <div className={styles.shell}>
      <header className={styles.nav}>
        <Link to="/" className={styles.brand} aria-label="img.li">
          <BrandLockup />
        </Link>
        <div className={styles.right}>
          <button type="button" className={styles.themeBtn} title={t('nav.toggleTheme')} onClick={toggleTheme}>
            {theme === 'light' ? '◐' : '◑'}
          </button>
          <LangToggle />
          <Link to="/" className={styles.linkBtn}>
            {t('share.uploadCta')}
          </Link>
        </div>
      </header>

      <main className={styles.main}>
        {q.isLoading && <div className={styles.msg}>{t('share.loading')}</div>}
        {notFound && (
          <div className={styles.msgBox}>
            <div className={styles.kicker}>NOT FOUND</div>
            <h1 className={styles.title}>{t('share.notFoundTitle')}</h1>
            <p className={styles.desc}>{t('share.notFoundDesc')}</p>
            <Link to="/">
              <Button variant="primary">{t('share.uploadCta')}</Button>
            </Link>
          </div>
        )}
        {q.isError && !notFound && (
          <div className={styles.msg}>{t('share.loadFailed')}</div>
        )}
        {data && needPw && (
          <div className={styles.msgBox}>
            <div className={styles.kicker}>PASSWORD</div>
            <h1 className={styles.title}>{t('share.passwordTitle')}</h1>
            <p className={styles.desc}>{t('share.passwordHint')}</p>
            <form
              className={styles.pwForm}
              onSubmit={(e) => {
                e.preventDefault()
                unlock.mutate(
                  { key, password: pw },
                  {
                    onError: () => {
                      /* toast via mutation error display below */
                    },
                  },
                )
              }}
            >
              <Input
                type="password"
                label={t('share.passwordPlaceholder')}
                value={pw}
                onChange={(e) => setPw(e.target.value)}
                autoComplete="current-password"
              />
              {unlock.isError && (
                <p className={styles.desc}>
                  {unlock.error instanceof ApiError && unlock.error.httpStatus === 401
                    ? t('share.passwordWrong')
                    : t('share.loadFailed')}
                </p>
              )}
              <Button variant="primary" type="submit" disabled={!pw.trim() || unlock.isPending}>
                {t('share.passwordSubmit')}
              </Button>
            </form>
          </div>
        )}
        {data && !needPw && (
          <div className={styles.card}>
            <div className={styles.previewWrap}>
              <img
                className={styles.preview}
                src={data.links.url}
                alt={data.name}
              />
            </div>
            <div className={styles.meta}>
              <h1 className={styles.name}>{data.name}</h1>
              <div className={styles.stats}>
                {data.width > 0 && data.height > 0 && (
                  <span>
                    {data.width}×{data.height}
                  </span>
                )}
                {data.size > 0 && (
                  <>
                    <span className={styles.dot}>·</span>
                    <span>{formatBytes(data.size)}</span>
                  </>
                )}
                {expiryLabel && (
                  <>
                    <span className={styles.dot}>·</span>
                    <span>{t('share.expires', { date: expiryLabel })}</span>
                  </>
                )}
                {!!data.max_views && data.max_views > 0 && (
                  <>
                    <span className={styles.dot}>·</span>
                    <span>
                      {t('images.maxViewsUsed', {
                        used: data.views_served ?? 0,
                        max: data.max_views,
                      })}
                    </span>
                  </>
                )}
              </div>
              <div className={styles.actions}>
                <Button
                  variant="primary"
                  onClick={() => copyText(data.links.url, t('share.copyUrl'))}
                >
                  {t('share.copyUrl')}
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => copyText(data.links.markdown, t('share.copyMarkdown'))}
                >
                  {t('share.copyMarkdown')}
                </Button>
                {(data.share_url || data.links.share_url) && (
                  <Button
                    variant="secondary"
                    onClick={() =>
                      copyText(data.share_url || data.links.share_url || '', t('share.copyShare'))
                    }
                  >
                    {t('share.copyShare')}
                  </Button>
                )}
              </div>
              <pre className={styles.urlLine}>{data.links.url}</pre>
            </div>
          </div>
        )}
      </main>
    </div>
  )
}
