import { useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { useForgotPassword } from '../../api/hooks'
import { useT } from '../../i18n'
import { Button } from '../../ui/Button'
import { Input } from '../../ui/Input'
import { AuthShell } from './AuthShell'

const EMAIL_RE = /^\S+@\S+\.\S+$/

export function ForgotPasswordPage() {
  const { t } = useT()
  const [email, setEmail] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [sent, setSent] = useState(false)
  const forgot = useForgotPassword()

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!EMAIL_RE.test(email)) return setError(t('auth.errEmailInvalid'))
    setError(null)
    // 后端恒 200(防枚举);网络错才可能失败,也按已发出展示
    forgot.mutate({ email: email.trim() }, { onSettled: () => setSent(true) })
  }

  return (
    <AuthShell>
      <div className="mb-2.5 font-mono text-[11px] tracking-[0.14em] text-muted">{t('auth.resetPasswordKicker')}</div>
      <h1 className="mb-6 mt-0 text-2xl font-bold tracking-[-0.015em]">{t('auth.forgotPasswordTitle')}</h1>
      {sent ? (
        <p className="mt-2 mb-0 text-[13.5px] leading-[1.8] text-muted">{t('auth.forgotSent')}</p>
      ) : (
        <form className="flex flex-col gap-3.5" onSubmit={submit} noValidate>
          <Input
            label={t('auth.email')}
            type="email"
            placeholder={t('auth.emailPlaceholder')}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          {error && <div className="animate-[fadeIn_0.15s] text-xs text-err">{error}</div>}
          <Button variant="primary" type="submit" className="mt-1 py-3 text-[13.5px]" disabled={forgot.isPending}>
            {t('auth.sendResetEmail')}
          </Button>
        </form>
      )}
      <Link to="/login" className="mt-[22px] inline-block text-sm-plus text-muted hover:text-ink">
        {t('auth.backToLogin')}
      </Link>
    </AuthShell>
  )
}
