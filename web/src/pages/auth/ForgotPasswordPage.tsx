import { useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { useForgotPassword } from '../../api/hooks'
import { useT } from '../../i18n'
import { Button } from '../../ui/Button'
import { Input } from '../../ui/Input'
import { AuthShell } from './AuthShell'
import styles from './AuthPage.module.css'

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
      <div className={styles.kicker}>{t('auth.resetPasswordKicker')}</div>
      <h1 className={styles.heading}>{t('auth.forgotPasswordTitle')}</h1>
      {sent ? (
        <p className={styles.flowText}>{t('auth.forgotSent')}</p>
      ) : (
        <form className={styles.fields} onSubmit={submit} noValidate>
          <Input
            label={t('auth.email')}
            type="email"
            placeholder={t('auth.emailPlaceholder')}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          {error && <div className={styles.error}>{error}</div>}
          <Button variant="primary" type="submit" className={styles.submit} disabled={forgot.isPending}>
            {t('auth.sendResetEmail')}
          </Button>
        </form>
      )}
      <Link to="/login" className={styles.flowLink}>{t('auth.backToLogin')}</Link>
    </AuthShell>
  )
}
