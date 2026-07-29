import { useMemo, useState } from 'react'
import { useConfig, useCreateToken, useRevokeToken, useTokens } from '../../api/hooks'
import { useT } from '../../i18n'
import { copyText } from '../../lib/copy'
import { formatDate } from '../../lib/format'
import { Button } from '../../ui/Button'
import { InlineConfirm } from '../../ui/inlineConfirm'
import { Modal } from '../../ui/Modal'
import styles from './SettingsPage.module.css'
import own from './TokensTab.module.css'
import { buildIntegrationSnippets, type SnippetKind } from './tokenSnippets'

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
      <div className={own.headRow}>
        <div className={styles.kicker}>{t('settings.tokensKicker')}</div>
        <Button variant="primary" onClick={() => setShowNew(true)}>
          {t('settings.generateToken')}
        </Button>
      </div>

      {fresh && (
        <div className={own.freshBox}>
          <div className={own.freshTitle}>{t('settings.freshTokenTitle')}</div>
          <div className={own.freshRow}>
            <span className={own.freshText}>{fresh}</span>
            <button type="button" className={own.freshCopy} onClick={() => copyText(fresh, t('settings.copyLabelToken'))}>
              {t('settings.copy')}
            </button>
            <button type="button" className={own.freshClose} onClick={() => setFresh(null)}>
              ×
            </button>
          </div>
          <p className={own.freshHint}>{t('settings.freshSnippetHint')}</p>
        </div>
      )}

      <div className={own.table}>
        <div className={own.tHead}>
          <span>{t('settings.colName')}</span><span>{t('settings.colToken')}</span><span>{t('settings.colScope')}</span><span>{t('settings.colCreated')}</span><span>{t('settings.colLastUsed')}</span><span></span>
        </div>
        {(tokens.data ?? []).map((tk) => (
          <div key={tk.id} className={own.tRow}>
            <span className={own.tName}>{tk.name}</span>
            <span className={own.tMask}>bl_····························</span>
            <span className={[own.scope, tk.scope === 'full' && own.scopeFull].filter(Boolean).join(' ')}>
              {tk.scope.toUpperCase()}
            </span>
            <span className={own.tMuted}>{formatDate(tk.created_at)}</span>
            <span className={own.tMuted}>{tk.last_used_at ? formatDate(tk.last_used_at) : t('settings.neverUsed')}</span>
            <InlineConfirm
              label={t('settings.revoke')}
              confirmLabel={t('settings.confirmRevoke')}
              onConfirm={() => revoke.mutate(tk.id)}
              disabled={revoke.isPending}
            />
          </div>
        ))}
        {tokens.data?.length === 0 && <div className={own.empty}>{t('settings.emptyTokens')}</div>}
      </div>

      <div className={own.confSection}>
        <div className={styles.kicker}>{t('settings.clientConfigKicker')}</div>
        <p className={own.confNote}>
          {t('settings.clientConfigNote', { base: baseURL || '…' })}
        </p>
        <div className={own.confGrid}>
          {snippets.map((s) => (
            <div key={s.kind} className={own.confCard}>
              <div className={own.confHead}>
                <span className={own.confName}>{titleByKind[s.kind]}</span>
                <button
                  type="button"
                  className={own.confCopy}
                  onClick={() => copyText(s.text, copyLabelByKind[s.kind])}
                >
                  {t('settings.copyConfig')}
                </button>
              </div>
              <pre className={own.confPre}>{s.text}</pre>
            </div>
          ))}
        </div>
      </div>

      <Modal open={showNew} onClose={() => setShowNew(false)} width={400}>
        <div className={styles.kicker}>NEW TOKEN</div>
        <div className={own.modalTitle}>{t('settings.newTokenTitle')}</div>
        <div className={styles.field}>
          <label className={styles.label} htmlFor="tk-name">{t('settings.colName')}</label>
          <input
            id="tk-name"
            className={styles.input}
            placeholder={t('settings.tokenNamePlaceholder')}
            value={name}
            onChange={(e) => {
              setName(e.target.value)
              setNameErr(false)
            }}
          />
          {nameErr && <div className={styles.errLine}>{t('settings.errNameRequired')}</div>}
        </div>
        <div className={styles.field}>
          <span className={styles.label}>{t('settings.scope')}</span>
          <button
            type="button"
            className={[own.scopeOpt, scope === 'upload' && own.scopeOptActive].filter(Boolean).join(' ')}
            onClick={() => setScope('upload')}
          >
            {t('settings.scopeUpload')}
            <span className={own.scopeSub}>{t('settings.scopeUploadSub')}</span>
          </button>
          <button
            type="button"
            className={[own.scopeOpt, scope === 'full' && own.scopeOptActive].filter(Boolean).join(' ')}
            onClick={() => setScope('full')}
          >
            {t('settings.scopeFull')}
            <span className={own.scopeSub}>{t('settings.scopeFullSub')}</span>
          </button>
        </div>
        <Button variant="primary" className={own.createBtn} disabled={create.isPending} onClick={createToken}>
          {t('settings.createToken')}
        </Button>
      </Modal>
    </div>
  )
}
