import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { ApiError } from '../../api/client'
import { useConfirmChangeEmail } from '../../api/hooks'
import { useT } from '../../i18n'
import { errorText } from '../../i18n/errorText'
import { AuthShell } from './AuthShell'

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
      <div className="mb-2.5 font-mono text-[11px] tracking-[0.14em] text-muted">{t('auth.confirmEmailKicker')}</div>
      <h1 className="mb-6 mt-0 text-2xl font-bold tracking-[-0.015em]">
        {state === 'pending'
          ? t('auth.confirmingEmail')
          : state === 'ok'
            ? t('auth.confirmEmailSuccess')
            : t('auth.confirmEmailFailed')}
      </h1>
      {state === 'fail' && <p className="mt-2 mb-0 text-[13.5px] leading-[1.8] text-muted">{msg}</p>}
      {state === 'ok' && (
        <p className="mt-2 mb-0 text-[13.5px] leading-[1.8] text-muted">{t('auth.confirmEmailSuccessHint')}</p>
      )}
      <p className="mt-2 mb-0 text-[13.5px] leading-[1.8] text-muted">
        <Link to="/login">{t('auth.goLogin')}</Link>
      </p>
    </AuthShell>
  )
}
