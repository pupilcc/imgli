import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import {
  useCheckSystemUpdate,
  useCleanupPreview,
  useCleanupRun,
  useSystemHealth,
  useSystemUpgrade,
  useSystemVersion,
  type DoctorLevel,
} from '../../../api/adminHooks'
import { useT } from '../../../i18n'
import { useGlobal } from '../../../store'
import { Button } from '../../../ui/Button'
import { PageHeader } from '../../../shell/PageHeader'
import { AdminQueryGate } from '../ui/AdminQueryGate'
import styles from './SystemPage.module.css'

const CHECKLIST_KEY = 'imgli_ops_checklist_dismissed'

function levelClass(level: DoctorLevel): string {
  if (level === 'fail') return styles.levelFail
  if (level === 'warn') return styles.levelWarn
  return styles.levelOk
}

function normalizeOrigin(raw: string): string {
  try {
    const u = new URL(raw.includes('://') ? raw : `http://${raw}`)
    return `${u.protocol}//${u.host}`
  } catch {
    return raw.replace(/\/$/, '')
  }
}

function isLocalishBase(base: string): boolean {
  try {
    const u = new URL(base)
    const h = u.hostname
    return h === 'localhost' || h === '127.0.0.1' || h === '::1'
  } catch {
    return /localhost|127\.0\.0\.1/.test(base)
  }
}

export function SystemPage() {
  const { t } = useT()
  const healthQ = useSystemHealth()
  const verQ = useSystemVersion()
  const checkUpdate = useCheckSystemUpdate()
  const doUpgrade = useSystemUpgrade()
  const previewCleanup = useCleanupPreview()
  const runCleanup = useCleanupRun()

  const [updateMsg, setUpdateMsg] = useState<string | null>(null)
  const [latestTag, setLatestTag] = useState<string | null>(null)
  const [cleanupMsg, setCleanupMsg] = useState<string | null>(null)
  const [checklistDismissed, setChecklistDismissed] = useState(
    () => typeof localStorage !== 'undefined' && localStorage.getItem(CHECKLIST_KEY) === '1',
  )

  const browserOrigin = typeof window !== 'undefined' ? window.location.origin : ''

  const runtime = healthQ.data?.runtime
  const baseNorm = runtime?.base_url ? normalizeOrigin(runtime.base_url) : ''
  const browserNorm = browserOrigin ? normalizeOrigin(browserOrigin) : ''
  const originMismatch = !!(baseNorm && browserNorm && baseNorm !== browserNorm)
  const localBase = runtime?.base_url ? isLocalishBase(runtime.base_url) : false

  const showChecklist = !checklistDismissed && (localBase || originMismatch)

  const suggestedEnv = useMemo(() => {
    if (!browserOrigin) return 'IMGLI_BASE_URL=https://your.domain'
    return `IMGLI_BASE_URL=${browserOrigin}\nIMGLI_TRUST_PROXY=true`
  }, [browserOrigin])

  const dismissChecklist = () => {
    localStorage.setItem(CHECKLIST_KEY, '1')
    setChecklistDismissed(true)
  }

  const onCheckUpdate = () => {
    setUpdateMsg(null)
    setLatestTag(null)
    checkUpdate.mutate(undefined, {
      onSuccess: (r) => {
        if (r.error) {
          setUpdateMsg(t('adminA.updateCheckFailed', { err: r.error }))
          return
        }
        if (r.update_available) {
          setLatestTag(r.latest ?? null)
          setUpdateMsg(t('adminA.updateAvailable', { latest: r.latest ?? '?' }))
        } else {
          setUpdateMsg(t('adminA.updateUpToDate', { current: r.current }))
        }
      },
    })
  }

  const onUpgrade = () => {
    if (!latestTag) return
    const hard = healthQ.data?.doctor.hard_fail
    const warnN = healthQ.data?.doctor.checks.filter((c) => c.level === 'warn').length ?? 0
    let conf = t('adminA.upgradeConfirm', { latest: latestTag })
    if (hard) conf = t('adminA.upgradePreflightFail') + '\n\n' + conf
    else if (warnN > 0) conf = t('adminA.upgradePreflightWarn', { n: warnN }) + '\n\n' + conf
    if (runtime?.install === 'docker') {
      conf = t('adminA.upgradeDockerBlocked') + '\n\n' + conf
    }
    if (!window.confirm(conf)) return
    doUpgrade.mutate(
      { confirm: true, tag: latestTag },
      {
        onSuccess: (r) => {
          if (r.mode === 'docker_blocked' || r.error) {
            setUpdateMsg(r.message || r.error || t('adminA.upgradeFailed'))
            return
          }
          setUpdateMsg(r.message || t('adminA.upgradeDone', { to: r.to ?? latestTag }))
          useGlobal.getState().pushToast(t('adminA.upgradeDoneToast'))
          // re-exec 后进程会换新二进制；稍后刷新版本信息
          window.setTimeout(() => {
            verQ.refetch()
            healthQ.refetch()
          }, 1500)
        },
      },
    )
  }

  const onPreviewCleanup = () => {
    setCleanupMsg(null)
    previewCleanup.mutate(
      { kinds: ['expired', 'trash', 'group_retention', 'group_force_age'] },
      {
        onSuccess: (r) => {
          const parts = (r.items ?? []).map((it) => `${it.kind}:${it.count}`)
          setCleanupMsg(t('adminA.cleanupPreviewResult', { detail: parts.join(' · ') || '—' }))
        },
      },
    )
  }

  const onRunCleanup = () => {
    if (!window.confirm(t('adminA.cleanupRunConfirm'))) return
    setCleanupMsg(null)
    runCleanup.mutate(
      { kinds: ['expired', 'trash', 'group_retention', 'group_force_age'], confirm: true, limit: 200 },
      {
        onSuccess: (r) => {
          const parts = (r.items ?? []).map((it) => `${it.kind}: deleted ${it.deleted ?? 0}`)
          setCleanupMsg(t('adminA.cleanupRunResult', { detail: parts.join(' · ') || '—' }))
          useGlobal.getState().pushToast(t('adminA.cleanupDoneToast'))
        },
      },
    )
  }

  return (
    <div>
      <PageHeader kicker="SYSTEM" title={t('adminA.systemTitle')} />

      {showChecklist && (
        <div className={styles.banner} role="status">
          <div className={styles.bannerTitle}>{t('adminA.checklistTitle')}</div>
          <div className={styles.bannerBody}>{t('adminA.checklistBody')}</div>
          <code className={styles.bannerCode}>{suggestedEnv}</code>
          <div className={styles.bannerActions}>
            <Button variant="primary" onClick={dismissChecklist}>
              {t('adminA.checklistDismiss')}
            </Button>
            <a
              href="https://github.com/yixian-huang/imgli/blob/main/docs/security-hardening.md#faq-reverse-proxy-loginregister-cross-site-rejected"
              target="_blank"
              rel="noreferrer"
            >
              {t('adminA.checklistDocs')}
            </a>
          </div>
        </div>
      )}

      <AdminQueryGate query={healthQ}>
        {(data) => (
          <>
            <section className={styles.section}>
              <div className={styles.sectionHead}>
                <h2 className={styles.h2}>{t('adminA.runtimeTitle')}</h2>
                <Button variant="secondary" disabled={healthQ.isFetching} onClick={() => healthQ.refetch()}>
                  {t('adminA.refreshHealth')}
                </Button>
              </div>
              <div className={styles.grid}>
                {[
                  { label: t('adminA.runningVersion'), value: data.runtime.version || verQ.data?.current || '…' },
                  { label: 'IMGLI_BASE_URL', value: data.runtime.base_url },
                  { label: 'trust_proxy', value: String(data.runtime.trust_proxy) },
                  { label: 'listen', value: data.runtime.listen },
                  { label: t('adminA.installShape'), value: data.runtime.install },
                  { label: 'data_dir', value: data.runtime.data_dir },
                  { label: t('adminA.requestHost'), value: data.runtime.request_host || '—' },
                  {
                    label: 'X-Forwarded-Proto',
                    value: data.runtime.forwarded_proto || '—',
                  },
                  {
                    label: 'X-Forwarded-For',
                    value: data.runtime.forwarded_for_set ? t('adminA.headerPresent') : '—',
                  },
                ].map((c) => (
                  <div key={c.label}>
                    <div className={styles.cellLabel}>{c.label}</div>
                    <div className={styles.cellValue}>{c.value}</div>
                  </div>
                ))}
              </div>
              {originMismatch ? (
                <div className={styles.mismatch}>
                  {t('adminA.originMismatch', { browser: browserNorm, base: baseNorm })}
                </div>
              ) : (
                <div className={styles.okNote}>{t('adminA.originMatch', { origin: browserNorm || baseNorm })}</div>
              )}
              {data.runtime.install === 'docker' && (
                <p className={styles.hint}>{t('adminA.dockerUpgradeHint')}</p>
              )}
            </section>

            <section className={styles.section}>
              <div className={styles.sectionHead}>
                <h2 className={styles.h2}>{t('adminA.doctorTitle')}</h2>
                {data.doctor.hard_fail ? (
                  <span className={`${styles.level} ${styles.levelFail}`}>FAIL</span>
                ) : (
                  <span className={`${styles.level} ${styles.levelOk}`}>OK</span>
                )}
              </div>
              <p className={styles.hint}>{t('adminA.doctorHint')}</p>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>{t('adminA.doctorCheck')}</th>
                    <th>{t('adminA.doctorLevel')}</th>
                    <th>{t('adminA.doctorMessage')}</th>
                  </tr>
                </thead>
                <tbody>
                  {data.doctor.checks.map((c) => (
                    <tr key={c.name + c.message}>
                      <td className={styles.mono}>{c.name}</td>
                      <td>
                        <span className={`${styles.level} ${levelClass(c.level)}`}>{c.level}</span>
                      </td>
                      <td>{c.message}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          </>
        )}
      </AdminQueryGate>

      <section className={styles.section}>
        <h2 className={styles.h2}>{t('adminA.upgradeSection')}</h2>
        <p className={styles.hint}>{t('adminA.upgradeSectionHint')}</p>
        <div className={styles.row}>
          <code className={styles.mono}>{verQ.data?.current ?? healthQ.data?.runtime.version ?? '…'}</code>
          <Button variant="secondary" disabled={checkUpdate.isPending} onClick={onCheckUpdate}>
            {t('adminA.checkUpdate')}
          </Button>
          {latestTag && runtime?.install !== 'docker' && (
            <Button variant="primary" disabled={doUpgrade.isPending} onClick={onUpgrade}>
              {t('adminA.upgradeTo', { latest: latestTag })}
            </Button>
          )}
        </div>
        {runtime?.install === 'docker' && (
          <code className={styles.bannerCode}>{t('adminA.dockerRedeploySnippet')}</code>
        )}
        {updateMsg && <div className={styles.msg}>{updateMsg}</div>}
      </section>

      <section className={styles.section}>
        <h2 className={styles.h2}>{t('adminA.cleanupSection')}</h2>
        <p className={styles.hint}>{t('adminA.cleanupHint')}</p>
        <div className={styles.row}>
          <Button variant="secondary" disabled={previewCleanup.isPending} onClick={onPreviewCleanup}>
            {t('adminA.cleanupPreview')}
          </Button>
          <Button variant="primary" disabled={runCleanup.isPending} onClick={onRunCleanup}>
            {t('adminA.cleanupRun')}
          </Button>
        </div>
        {cleanupMsg && <div className={styles.msg}>{cleanupMsg}</div>}
      </section>

      <section className={styles.section}>
        <h2 className={styles.h2}>{t('adminA.opsLinksTitle')}</h2>
        <div className={styles.linkRow}>
          <Link to="/admin/policies">{t('adminA.linkPoliciesMigrate')}</Link>
          <Link to="/admin/logs">{t('nav.logs')}</Link>
          <a href="https://github.com/yixian-huang/imgli/blob/main/docs/backup.md" target="_blank" rel="noreferrer">
            {t('adminA.linkBackupDocs')}
          </a>
        </div>
        <p className={styles.hint}>{t('adminA.backupHint')}</p>
      </section>
    </div>
  )
}
