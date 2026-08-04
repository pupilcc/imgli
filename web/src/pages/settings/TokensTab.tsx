import { useMemo, useState } from 'react'
import { useConfig, useCreateToken, useRevokeToken, useTokens } from '../../api/hooks'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { copyText } from '../../lib/copy'
import { formatDate } from '../../lib/format'
import { Button } from '../../ui/Button'
import { InlineConfirm } from '../../ui/InlineConfirm'
import { Modal } from '../../ui/Modal'
import { StepGuide } from '../../ui/StepGuide'
import { buildIntegrationSnippets, type SnippetKind } from './tokenSnippets'

const kicker = 'mb-3 font-mono text-2xs tracking-[0.14em] text-muted'
const field = 'flex flex-col gap-1.5'
const label = 'text-xs font-semibold text-muted'
const input =
  'rounded-sm border border-border bg-bg px-3 py-[9px] font-inherit text-[13px] text-ink outline-none focus:border-muted'
const errLine = 'animate-[fadeIn_0.15s] text-xs text-err'
const tableCols =
  'grid grid-cols-[1.2fr_1.5fr_0.8fr_0.9fr_0.9fr_auto] items-center gap-3 border-b border-border px-4 py-[11px] max-md:grid-cols-[1fr_0.8fr_auto]'
const hideOnMobile =
  'max-md:[&>*:nth-child(2)]:hidden max-md:[&>*:nth-child(4)]:hidden max-md:[&>*:nth-child(5)]:hidden'

export function TokensTab() {
  const { t } = useT()
  const tokens = useTokens()
  const create = useCreateToken()
  const revoke = useRevokeToken()
  const cfg = useConfig()
  const [showNew, setShowNew] = useState(false)
  const [name, setName] = useState('')
  const [scope, setScope] = useState<'upload' | 'full'>('upload')
  const [fresh, setFresh] = useState<string | null>(null)
  const [nameErr, setNameErr] = useState(false)

  const baseURL =
    (cfg.data?.base_url && cfg.data.base_url.trim()) ||
    (typeof window !== 'undefined' ? window.location.origin : '')

  // Plain token only while the create-once banner is open; otherwise placeholder label.
  const tokenForSnippet = fresh ?? t('settings.tokenPlaceholder')
  const snippets = useMemo(
    () => buildIntegrationSnippets(baseURL, tokenForSnippet),
    [baseURL, tokenForSnippet],
  )

  const titleByKind: Record<SnippetKind, string> = {
    curl: 'curl',
    picgo: 'PicGo',
    sharex: 'ShareX',
    cli: t('settings.cliName'),
  }
  const copyLabelByKind: Record<SnippetKind, string> = {
    curl: t('settings.copyLabelCurl'),
    picgo: t('settings.copyLabelPicgo'),
    sharex: t('settings.copyLabelSharex'),
    cli: t('settings.copyLabelCli'),
  }

  function createToken() {
    if (!name.trim()) return setNameErr(true)
    create.mutate(
      { name: name.trim(), scope },
      {
        onSuccess: (tk) => {
          setFresh(tk.token ?? null)
          setShowNew(false)
          setName('')
        },
      },
    )
  }

  return (
    <div>
      <div className="mb-2 flex items-center justify-between [&>div]:mb-0">
        <div className={kicker}>{t('settings.tokensKicker')}</div>
        <Button variant="primary" onClick={() => setShowNew(true)}>
          {t('settings.generateToken')}
        </Button>
      </div>
      <StepGuide
        data-testid="tokens-onboarding"
        kicker={t('settings.tokensOnboardingKicker')}
        steps={[
          t('settings.tokensOnboardingStep1'),
          t('settings.tokensOnboardingStep2'),
          t('settings.tokensOnboardingStep3'),
        ]}
      />

      {fresh && (
        <div className="mb-3 animate-[fadeIn_0.2s] rounded-sm border border-ink bg-surface px-4 py-3.5">
          <div className="mb-2 text-xs font-bold">{t('settings.freshTokenTitle')}</div>
          <div className="flex items-center overflow-hidden rounded-sm border border-border">
            <span className="min-w-0 flex-1 overflow-hidden bg-soft px-3 py-2 font-mono text-[11.5px] text-ellipsis whitespace-nowrap">
              {fresh}
            </span>
            <button
              type="button"
              className="min-w-16 shrink-0 cursor-pointer border-0 border-l border-border bg-surface px-3.5 py-2 text-[11.5px] font-bold text-ink hover:bg-soft"
              onClick={() => copyText(fresh, t('settings.copyLabelToken'))}
            >
              {t('settings.copy')}
            </button>
            <button
              type="button"
              className="shrink-0 cursor-pointer border-0 border-l border-border bg-surface px-3 py-2 text-[13px] leading-none text-muted hover:text-ink"
              onClick={() => setFresh(null)}
            >
              ×
            </button>
          </div>
          <p className="mt-2.5 mb-0 text-[11.5px] leading-normal text-muted">{t('settings.freshSnippetHint')}</p>
        </div>
      )}

      <div className="overflow-hidden rounded-sm border border-border bg-surface">
        <div
          className={cn(
            tableCols,
            hideOnMobile,
            'bg-soft py-[9px] font-mono text-2xs tracking-[0.1em] text-muted',
          )}
        >
          <span>{t('settings.colName')}</span>
          <span>{t('settings.colToken')}</span>
          <span>{t('settings.colScope')}</span>
          <span>{t('settings.colCreated')}</span>
          <span>{t('settings.colLastUsed')}</span>
          <span />
        </div>
        {(tokens.data ?? []).map((tk) => (
          <div key={tk.id} className={cn(tableCols, hideOnMobile)}>
            <span className="truncate text-[13px] font-bold">{tk.name}</span>
            <span className="font-mono text-[11px] text-muted">bl_····························</span>
            <span
              className={cn(
                'justify-self-start rounded-[2px] border border-border px-[7px] py-0.5 font-mono text-[9.5px] tracking-[0.06em] text-muted',
                tk.scope === 'full' && 'border-warn text-warn',
              )}
            >
              {tk.scope.toUpperCase()}
            </span>
            <span className="font-mono text-xs-plus text-muted">{formatDate(tk.created_at)}</span>
            <span className="font-mono text-xs-plus text-muted">
              {tk.last_used_at ? formatDate(tk.last_used_at) : t('settings.neverUsed')}
            </span>
            <InlineConfirm
              label={t('settings.revoke')}
              confirmLabel={t('settings.confirmRevoke')}
              onConfirm={() => revoke.mutate(tk.id)}
              disabled={revoke.isPending}
            />
          </div>
        ))}
        {tokens.data?.length === 0 && (
          <div className="px-7 py-7 text-center text-sm-plus text-muted">{t('settings.emptyTokens')}</div>
        )}
      </div>

      <div className="mt-7">
        <div className={kicker}>{t('settings.clientConfigKicker')}</div>
        <p className="mb-3 mt-0 text-xs leading-normal text-muted">
          {t('settings.clientConfigNote', { base: baseURL || '…' })}
        </p>
        <div className="grid grid-cols-2 gap-3.5 max-md:grid-cols-1">
          {snippets.map((s) => (
            <div key={s.kind} className="overflow-hidden rounded-sm border border-border bg-surface">
              <div className="flex items-center justify-between border-b border-border bg-soft px-3.5 py-[9px]">
                <span className="text-xs font-bold">{titleByKind[s.kind]}</span>
                <button
                  type="button"
                  className="cursor-pointer border-0 bg-transparent text-[11px] font-semibold text-muted underline hover:text-ink"
                  onClick={() => copyText(s.text, copyLabelByKind[s.kind])}
                >
                  {t('settings.copyConfig')}
                </button>
              </div>
              <pre className="m-0 overflow-auto bg-bg p-3.5 font-mono text-xs-plus leading-[1.7] text-ink">
                {s.text}
              </pre>
            </div>
          ))}
        </div>
      </div>

      <Modal open={showNew} onClose={() => setShowNew(false)} width={400}>
        <div className={kicker}>NEW TOKEN</div>
        <div className="mb-3.5 text-base font-bold">{t('settings.newTokenTitle')}</div>
        <div className={field}>
          <label className={label} htmlFor="tk-name">
            {t('settings.colName')}
          </label>
          <input
            id="tk-name"
            className={input}
            placeholder={t('settings.tokenNamePlaceholder')}
            value={name}
            onChange={(e) => {
              setName(e.target.value)
              setNameErr(false)
            }}
          />
          {nameErr && <div className={errLine}>{t('settings.errNameRequired')}</div>}
        </div>
        <div className={`${field} mt-3.5`}>
          <span className={label}>{t('settings.scope')}</span>
          <button
            type="button"
            className={cn(
              'mb-2 w-full cursor-pointer rounded-sm border border-border bg-surface px-3.5 py-[11px] text-left text-sm-plus font-semibold text-ink hover:bg-soft',
              scope === 'upload' && 'border-ink bg-soft',
            )}
            onClick={() => setScope('upload')}
          >
            {t('settings.scopeUpload')}
            <span className="mt-0.5 block text-[11px] font-normal text-muted">{t('settings.scopeUploadSub')}</span>
          </button>
          <button
            type="button"
            className={cn(
              'mb-2 w-full cursor-pointer rounded-sm border border-border bg-surface px-3.5 py-[11px] text-left text-sm-plus font-semibold text-ink hover:bg-soft',
              scope === 'full' && 'border-ink bg-soft',
            )}
            onClick={() => setScope('full')}
          >
            {t('settings.scopeFull')}
            <span className="mt-0.5 block text-[11px] font-normal text-muted">{t('settings.scopeFullSub')}</span>
          </button>
        </div>
        <Button
          variant="primary"
          className="mt-1 w-full py-[11px] text-[13px]"
          disabled={create.isPending}
          onClick={createToken}
        >
          {t('settings.createToken')}
        </Button>
      </Modal>
    </div>
  )
}
