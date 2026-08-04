import { useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router'
import { ApiError } from '../../api/client'
import { sessionKey, useVerifyEmail } from '../../api/hooks'
import { useT } from '../../i18n'
import { errorText } from '../../i18n/errorText'
import { AuthShell } from './AuthShell'

export function VerifyEmailPage() {
  const { t } = useT()
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''
  const [state, setState] = useState<'pending' | 'ok' | 'fail'>('pending')
  const [msg, setMsg] = useState('')
  const verify = useVerifyEmail()
  const qc = useQueryClient()
  const fired = useRef(false)

  useEffect(() => {
    if (fired.current) return
    fired.current = true
    if (!token) {
      setState('fail')
      setMsg(t('auth.errTokenInvalid'))
      return
    }
    verify.mutate(
      { token },
      {
        onSuccess: () => {
          qc.invalidateQueries({ queryKey: sessionKey })
          setState('ok')
        },
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
  }, [token, verify, qc, t])

  return (
    <AuthShell>
      <div className="mb-2.5 font-mono text-[11px] tracking-[0.14em] text-muted">{t('auth.verifyEmailKicker')}</div>
      <h1 className="mb-6 mt-0 text-2xl font-bold tracking-[-0.015em]">
        {state === 'pending'
          ? t('auth.verifying')
          : state === 'ok'
            ? t('auth.verifySuccess')
            : t('auth.verifyFailed')}
      </h1>
      {state === 'fail' && (
        <p className="mt-2 mb-0 text-[13.5px] leading-[1.8] text-muted">
          {msg}
          {t('auth.verifyFailHint')}
        </p>
      )}
      {state === 'ok' && <p className="mt-2 mb-0 text-[13.5px] leading-[1.8] text-muted">{t('auth.verifyOkText')}</p>}
      <Link to="/login" className="mt-[22px] inline-block text-sm-plus text-muted hover:text-ink">
        {t('auth.backToLogin')}
      </Link>
    </AuthShell>
  )
}
