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
      <div className="mb-2.5 font-mono text-[11px] tracking-[0.14em] text-muted">{t('auth.newPasswordKicker')}</div>
      <h1 className="mb-6 mt-0 text-2xl font-bold tracking-[-0.015em]">{t('auth.setNewPassword')}</h1>
      {done ? (
        <p className="mt-2 mb-0 text-[13.5px] leading-[1.8] text-muted">{t('auth.passwordResetDone')}</p>
      ) : (
        <form className="flex flex-col gap-3.5" onSubmit={submit} noValidate>
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
          {error && <div className="animate-[fadeIn_0.15s] text-xs text-err">{error}</div>}
          <Button variant="primary" type="submit" className="mt-1 py-3 text-[13.5px]" disabled={reset.isPending}>
            {t('auth.resetPassword')}
          </Button>
        </form>
      )}
      <Link to="/login" className="mt-[22px] inline-block text-sm-plus text-muted hover:text-ink">
        {t('auth.backToLogin')}
      </Link>
    </AuthShell>
  )
}
