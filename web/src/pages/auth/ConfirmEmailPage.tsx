import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { ApiError } from '../../api/client'
import { useConfirmChangeEmail } from '../../api/hooks'
import { useT } from '../../i18n'
import { errorText } from '../../i18n/errorText'
import { AuthShell } from './AuthShell'
import styles from './AuthPage.module.css'

/** 换绑邮箱确认页：/confirm-email?token= */
export function ConfirmEmailPage() {
  const { t } = useT()
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''
  const [state, setState] = useState<'pending' | 'ok' | 'fail'>('pending')
  const [msg, setMsg] = useState('')
  const confirm = useConfirmChangeEmail()
  const fired = useRef(false)

  useEffect(() => {
    if (fired.current) return
    fired.current = true
    if (!token) {
      setState('fail')
      setMsg(t('auth.errTokenInvalid'))
      return
    }
    confirm.mutate(
      { token },
      {
        onSuccess: () => setState('ok'),
        onError: (err) => {
          setState('fail')
          setMsg(
            err instanceof ApiError
              ? errorText(err.code, err.message)
              : t('auth.requestFailed'),
          )
        },
      },
    )
  }, [token, confirm, t])

  return (
    <AuthShell>
      <div className={styles.kicker}>{t('auth.confirmEmailKicker')}</div>
      <h1 className={styles.heading}>
        {state === 'pending'
          ? t('auth.confirmingEmail')
          : state === 'ok'
            ? t('auth.confirmEmailSuccess')
            : t('auth.confirmEmailFailed')}
      </h1>
      {state === 'fail' && <p className={styles.flowText}>{msg}</p>}
      {state === 'ok' && (
        <p className={styles.flowText}>{t('auth.confirmEmailSuccessHint')}</p>
      )}
      <p className={styles.flowText}>
        <Link to="/login">{t('auth.goLogin')}</Link>
      </p>
    </AuthShell>
  )
}
