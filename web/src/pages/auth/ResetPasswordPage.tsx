import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router'
import { ApiError } from '../../api/client'
import { useResetPassword } from '../../api/hooks'
import { useT } from '../../i18n'
import { errorText } from '../../i18n/errorText'
import { STRONG_RE } from '../../lib/password'
import { Button } from '../../ui/Button'
import { Input } from '../../ui/Input'
import { AuthShell } from './AuthShell'
import styles from './AuthPage.module.css'

export function ResetPasswordPage() {
  const { t } = useT()
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''
  const [pwd, setPwd] = useState('')
  const [pwd2, setPwd2] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [done, setDone] = useState(false)
  const reset = useResetPassword()

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!token) return setError(t('auth.errTokenInvalid'))
    if (!STRONG_RE.test(pwd)) return setError(t('auth.errPasswordWeak'))
    if (pwd !== pwd2) return setError(t('auth.errPasswordMismatch'))
    setError(null)
    reset.mutate(
      { token, password: pwd },
      {
        onSuccess: () => setDone(true),
        onError: (err) =>
          setError(
            err instanceof ApiError
              ? errorText(err.code, err.message)
              : t('auth.requestFailed'),
          ),
      },
    )
  }

  return (
    <AuthShell>
      <div className={styles.kicker}>{t('auth.newPasswordKicker')}</div>
      <h1 className={styles.heading}>{t('auth.setNewPassword')}</h1>
      {done ? (
        <p className={styles.flowText}>{t('auth.passwordResetDone')}</p>
      ) : (
        <form className={styles.fields} onSubmit={submit} noValidate>
          <Input
            label={t('auth.newPassword')}
            type="password"
            placeholder={t('auth.passwordPlaceholderReg')}
            value={pwd}
            onChange={(e) => setPwd(e.target.value)}
          />
          <Input
            label={t('auth.confirmNewPassword')}
            type="password"
            placeholder={t('auth.passwordPlaceholderAgain')}
            value={pwd2}
            onChange={(e) => setPwd2(e.target.value)}
          />
          {error && <div className={styles.error}>{error}</div>}
          <Button variant="primary" type="submit" className={styles.submit} disabled={reset.isPending}>
            {t('auth.resetPassword')}
          </Button>
        </form>
      )}
      <Link to="/login" className={styles.flowLink}>{t('auth.backToLogin')}</Link>
    </AuthShell>
  )
}
