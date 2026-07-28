import { useState } from 'react'
import { useCreateToken, useRevokeToken, useTokens } from '../../api/hooks'
import { useT } from '../../i18n'
import { copyText } from '../../lib/copy'
import { formatDate } from '../../lib/format'
import { Button } from '../../ui/Button'
import { InlineConfirm } from '../../ui/InlineConfirm'
import { Modal } from '../../ui/Modal'
import styles from './SettingsPage.module.css'
import own from './TokensTab.module.css'

export function TokensTab() {
  const { t } = useT()
  const tokens = useTokens()
  const create = useCreateToken()
  const revoke = useRevokeToken()
  const [showNew, setShowNew] = useState(false)
  const [name, setName] = useState('')
  const [scope, setScope] = useState<'upload' | 'full'>('upload')
  const [fresh, setFresh] = useState<string | null>(null)
  const [nameErr, setNameErr] = useState(false)

  const origin = window.location.origin
  const tokenPh = t('settings.tokenPlaceholder')
  const picgo = `{
  "picBed": {
    "current": "lankong",
    "lankong": {
      "lskyProVersion": "V2",
      "server": "${origin}",
      "token": "Bearer <${tokenPh}>"
    }
  }
}`
  const typora = `sh -c 'curl -s -X POST \\
  -H "Authorization: Bearer <${tokenPh}>" \\
  -F "file=@$1" \\
  ${origin}/api/v1/upload \\
  | python3 -c "import sys,json;d=json.load(sys.stdin)[\\"data\\"];print(\\"Upload Success:\\");print(d[\\"links\\"][\\"url\\"])"' _`

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
        <div className={own.confGrid}>
          <div className={own.confCard}>
            <div className={own.confHead}>
              <span className={own.confName}>PicGo</span>
              <button type="button" className={own.confCopy} onClick={() => copyText(picgo, t('settings.copyLabelPicgo'))}>
                {t('settings.copyConfig')}
              </button>
            </div>
            <pre className={own.confPre}>{picgo}</pre>
          </div>
          <div className={own.confCard}>
            <div className={own.confHead}>
              <span className={own.confName}>{t('settings.typoraName')}</span>
              <button type="button" className={own.confCopy} onClick={() => copyText(typora, t('settings.copyLabelTypora'))}>
                {t('settings.copyCommand')}
              </button>
            </div>
            <pre className={own.confPre}>{typora}</pre>
          </div>
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
