import { useState, useEffect, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { ApiError } from '../../api/client'
import { useLogin, useRegister, useConfig } from '../../api/hooks'
import { useT } from '../../i18n'
import { errorText } from '../../i18n/errorText'
import { pickLocale } from '../../lib/locale'
import { STRONG_RE } from '../../lib/password'
import { safeNext } from '../../lib/safeNext'
import { useGlobal } from '../../store'
import { BrandLockup } from '../../ui/Brand'
import { Button } from '../../ui/Button'
import { Input } from '../../ui/Input'
import { LangToggle } from '../../ui/LangToggle'
import { Segmented } from '../../ui/Segmented'
import styles from './AuthPage.module.css'

type Mode = 'login' | 'reg'

const EMAIL_RE = /^\S+@\S+\.\S+$/

export function AuthPage() {
  const { t, lang } = useT()
  const [mode, setMode] = useState<Mode>('login')
  const [username, setUsername] = useState('')
  const [account, setAccount] = useState('')
  const [email, setEmail] = useState('')
  const [pwd, setPwd] = useState('')
  const [invite, setInvite] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState(false)
  const theme = useGlobal((s) => s.theme)
  const toggleTheme = useGlobal((s) => s.toggleTheme)
  const login = useLogin()
  const register = useRegister()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const afterAuth = safeNext(searchParams.get('next'), '/')
  const busy = login.isPending || register.isPending
  const isLogin = mode === 'login'
  const config = useConfig()
  const regClosed = config.data?.registration_mode === 'closed'
  const regInvite = config.data?.registration_mode === 'invite'
  const siteName = (config.data?.site_name || 'imgli').trim() || 'imgli'
  const registerNotice = pickLocale(config.data?.register_notice, lang)
  // 浅场景文案：?from=picgo|blog|private 或 utm_campaign（非多触点 funnel）
  const scenarioCopy = (() => {
    try {
      const q = new URLSearchParams(window.location.search)
      const key = (q.get('from') || q.get('utm_campaign') || '').toLowerCase()
      if (key === 'picgo' || key === 'sharex' || key === 'upic') return t('auth.scenarioPicgo')
      if (key === 'blog' || key === 'markdown' || key === 'typora') return t('auth.scenarioBlog')
      if (key === 'private' || key === 'team') return t('auth.scenarioPrivate')
    } catch {
      /* ignore */
    }
    return ''
  })()
  const helpURL = (config.data?.help_url || '').trim()
  const upgradeURL = (config.data?.upgrade_url || '').trim()
  // 注册关闭时强制回登录模式(含直接停在 reg 态的场景)
  useEffect(() => {
    if (regClosed && mode === 'reg') setMode('login')
  }, [regClosed, mode])

  function submit(e: FormEvent) {
    e.preventDefault()
    if (busy || done) return
    if (!isLogin && !username.trim()) return setError(t('auth.errUsernameRequired'))
    if (isLogin) {
      if (!account.trim()) return setError(t('auth.errAccountRequired'))
      if (!pwd) return setError(t('auth.errPasswordRequired'))
    } else {
      if (!EMAIL_RE.test(email)) return setError(t('auth.errEmailInvalid'))
      if (!STRONG_RE.test(pwd)) return setError(t('auth.errPasswordWeak'))
      if (regInvite && !invite.trim()) return setError(t('auth.errInviteRequired'))
    }
    setError(null)
    const onSuccess = () => {
      setDone(true)
      setTimeout(() => navigate(afterAuth), 400)
    }
    const onError = (err: unknown) =>
      setError(
        err instanceof ApiError
          ? errorText(err.code, err.message)
          : t('auth.requestFailed'),
      )
    if (isLogin) login.mutate({ account: account.trim(), password: pwd }, { onSuccess, onError })
    else {
      const utm = {
        utm_source: searchParams.get('utm_source') || undefined,
        utm_medium: searchParams.get('utm_medium') || undefined,
        utm_campaign: searchParams.get('utm_campaign') || undefined,
      }
      let referer_host: string | undefined
      try {
        if (document.referrer) referer_host = new URL(document.referrer).hostname || undefined
      } catch {
        /* ignore */
      }
      register.mutate(
        {
          username: username.trim(),
          email: email.trim(),
          password: pwd,
          ...(regInvite ? { invite_code: invite.trim().toUpperCase() } : {}),
          ...utm,
          ...(referer_host ? { referer_host } : {}),
        },
        { onSuccess, onError },
      )
    }
  }

  const submitLabel = done
    ? t('auth.successRedirect')
    : busy
      ? t('auth.pleaseWait')
      : isLogin
        ? t('auth.submitLogin')
        : t('auth.submitRegister')

  return (
    <div className={styles.page}>
      <aside className={styles.brand}>
        <div className={styles.brandLogo}>
          <BrandLockup beta invert />
        </div>
        <div>
          <div className={styles.slogan}>{t('meta.slogan')}</div>
          <div className={styles.headline}>
            {t('auth.headlineLine1')}
            <br />
            {t('auth.headlineLine2')}
          </div>
        </div>
        <div className={styles.copyright}>
          {t('auth.copyright', { year: new Date().getFullYear(), site: siteName })}
        </div>
        <div className={styles.deco} />
      </aside>

      <main className={styles.formPane}>
        <div className={styles.topBar}>
          <button type="button" className={styles.themeBtn} title={t('nav.toggleTheme')} onClick={toggleTheme}>
            {theme === 'light' ? '◐' : '◑'}
          </button>
          <LangToggle />
        </div>
        <div className={styles.formBox}>
          <div className={styles.kicker}>{isLogin ? t('auth.signInKicker') : t('auth.createAccountKicker')}</div>
          <h1 className={styles.heading}>{isLogin ? t('auth.welcomeBack') : t('auth.createAccount')}</h1>
          {!isLogin && !regClosed && registerNotice && (
            <p className={styles.trialNote} data-testid="reg-trial-note">
              {registerNotice}
              {(helpURL || upgradeURL) && (
                <>
                  {' '}
                  {helpURL && (
                    <a href={helpURL} rel="noopener noreferrer">
                      {t('auth.helpLink')}
                    </a>
                  )}
                  {helpURL && upgradeURL ? ' · ' : null}
                  {upgradeURL && (
                    <a href={upgradeURL} rel="noopener noreferrer">
                      {t('auth.upgradeLink')}
                    </a>
                  )}
                </>
              )}
            </p>
          )}

          <div className={styles.switch}>
            {regClosed ? (
              <div className={styles.regClosed}>{t('auth.regClosed')}</div>
            ) : (
              <Segmented<Mode>
                options={[
                  { value: 'login', label: t('auth.login') },
                  { value: 'reg', label: t('auth.register') },
                ]}
                value={mode}
                onChange={(m) => {
                  setMode(m)
                  setError(null)
                }}
              />
            )}
          </div>

          <form className={styles.fields} onSubmit={submit} noValidate>
            {!isLogin && (
              <Input
                label={t('auth.username')}
                placeholder={t('auth.usernamePlaceholder')}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
            )}
            {isLogin ? (
              <Input
                label={t('auth.account')}
                placeholder={t('auth.accountPlaceholder')}
                value={account}
                onChange={(e) => setAccount(e.target.value)}
              />
            ) : (
              <Input
                label={t('auth.email')}
                type="email"
                placeholder={t('auth.emailPlaceholder')}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            )}
            <Input
              label={t('auth.password')}
              type="password"
              placeholder={isLogin ? t('auth.passwordPlaceholderLogin') : t('auth.passwordPlaceholderReg')}
              value={pwd}
              onChange={(e) => setPwd(e.target.value)}
            />
            {isLogin && (
              <div className={styles.forgotRow}>
                <Link to="/forgot-password" className={styles.forgotLink}>{t('auth.forgotPassword')}</Link>
              </div>
            )}
            {!isLogin && regInvite && (
              <Input
                label={t('auth.inviteCode')}
                placeholder={t('auth.inviteCodePlaceholder')}
                value={invite}
                onChange={(e) => setInvite(e.target.value)}
              />
            )}
            {error && <div className={styles.error}>{error}</div>}
            <Button
              variant="primary"
              type="submit"
              data-testid="auth-submit"
              className={styles.submit}
              disabled={busy}
            >
              {submitLabel}
            </Button>
            {isLogin && config.data?.oidc_enabled && (
              <a href="/api/v1/auth/oidc/start" className={styles.submit} style={{ display: 'block', textAlign: 'center', marginTop: 10 }}>
                <Button variant="secondary" type="button" style={{ width: '100%' }}>
                  {t('auth.oidcLogin')}
                </Button>
              </a>
            )}
          </form>
        </div>
      </main>
    </div>
  )
}
