import { useEffect, useRef, useState } from 'react'
import { ApiError } from '../../../api/client'
import { useAdminSettings, useTestModeration, useTestSMTP, useUpdateSettings } from '../../../api/adminHooks'
import { POSITIONS, type AdminSettings, type FooterGroup, type SiteAnnouncement } from '../../../api/types'
import { useT } from '../../../i18n'
import { errorText } from '../../../i18n/errorText'
import { PageHeader } from '../../../shell/PageHeader'
import { useGlobal } from '../../../store'
import { Button } from '../../../ui/Button'
import { Input } from '../../../ui/Input'
import { Segmented } from '../../../ui/Segmented'
import { Skeleton } from '../../../ui/Skeleton'
import { Toggle } from '../../../ui/Toggle'
import { AdminError } from '../ui/AdminError'
import { formatLexiconExport, mergeLexiconText, parseLexiconText } from './lexicon'
import styles from './SettingsPage.module.css'

type ModProvider = 'webhook' | 'aliyun' | 'tencent' | 'openai' | 'nsfwjs'

type SettingsTab = 'basic' | 'slots' | 'moderation' | 'ocr' | 'smtp' | 'hotlink' | 'processing'

const SETTINGS_TABS: {
  key: SettingsTab
  labelKey: 'basic' | 'slotsTab' | 'moderation' | 'ocrSection' | 'smtpSection' | 'hotlink' | 'processing'
}[] = [
  { key: 'basic', labelKey: 'basic' },
  { key: 'slots', labelKey: 'slotsTab' },
  { key: 'moderation', labelKey: 'moderation' },
  { key: 'ocr', labelKey: 'ocrSection' },
  { key: 'smtp', labelKey: 'smtpSection' },
  { key: 'hotlink', labelKey: 'hotlink' },
  { key: 'processing', labelKey: 'processing' },
]

const emptyAnn = (): SiteAnnouncement => ({
  enabled: false,
  text: '',
  link_url: '',
  link_label: '',
  dismissible: true,
  starts_at: '',
  ends_at: '',
})

interface FormState {
  siteName: string
  regMode: 'open' | 'invite' | 'closed'
  guestUpload: boolean
  plazaEnabled: boolean
  modEnabled: boolean
  modProvider: ModProvider
  modEndpoint: string
  modApiKey: string
  modAKID: string
  modAKSecret: string
  modRegion: string
  modThreshold: number
  modAction: 'pending' | 'rejected'
  loginSampleRate: number
  onPluginError: 'open' | 'review'
  notifyOnReject: boolean
  ocrEnabled: boolean
  ocrEndpoint: string
  ocrApiKey: string
  ocrKeywords: string
  ocrOnHit: 'review' | 'block'
  smtpHost: string
  smtpPort: number
  smtpUser: string
  smtpPassword: string
  smtpFrom: string
  smtpEnc: 'none' | 'starttls' | 'ssl'
  hotlinkEnabled: boolean
  hotlinkDomains: string
  hotlinkAllowEmpty: boolean
  twEnabled: boolean
  twText: string
  twPos: string
  twOpacity: number
  twSizeRatio: number
  maxEdge: number
  stripExif: boolean
  ann: SiteAnnouncement
  footerGroups: FooterGroup[]
  htmlHead: string
  htmlBodyEnd: string
}

const MOD_PROVIDERS: ModProvider[] = ['webhook', 'aliyun', 'tencent', 'openai', 'nsfwjs']

function formOf(s: AdminSettings): FormState {
  const tw = s.processing?.text_watermark
  const p = s.moderation.provider
  return {
    siteName: s.site_name,
    regMode: s.registration_mode === 'closed' ? 'closed' : s.registration_mode === 'invite' ? 'invite' : 'open',
    guestUpload: s.guest_upload_enabled,
    plazaEnabled: s.plaza_enabled,
    modEnabled: s.moderation.enabled,
    modProvider: MOD_PROVIDERS.includes(p as ModProvider) ? (p as ModProvider) : 'webhook',
    modEndpoint: s.moderation.endpoint,
    modApiKey: s.moderation.api_key,
    modAKID: s.moderation.access_key_id ?? '',
    modAKSecret: s.moderation.access_key_secret ?? '',
    modRegion: s.moderation.region ?? '',
    modThreshold: s.moderation.threshold,
    modAction: s.moderation.action === 'rejected' ? 'rejected' : 'pending',
    loginSampleRate:
      s.moderation.login_sample_rate != null && s.moderation.login_sample_rate >= 0
        ? s.moderation.login_sample_rate
        : 1,
    onPluginError: s.moderation.on_plugin_error === 'review' ? 'review' : 'open',
    notifyOnReject: !!s.moderation.notify_on_reject,
    ocrEnabled: s.moderation.ocr_keywords?.enabled ?? false,
    ocrEndpoint: s.moderation.ocr_keywords?.endpoint ?? '',
    ocrApiKey: s.moderation.ocr_keywords?.api_key ?? '',
    ocrKeywords: (s.moderation.ocr_keywords?.keywords ?? []).join('\n'),
    ocrOnHit: s.moderation.ocr_keywords?.on_hit === 'block' ? 'block' : 'review',
    smtpHost: s.smtp.host,
    smtpPort: s.smtp.port,
    smtpUser: s.smtp.username,
    smtpPassword: s.smtp.password,
    smtpFrom: s.smtp.from,
    smtpEnc: s.smtp.encryption === 'none' || s.smtp.encryption === 'ssl' ? s.smtp.encryption : 'starttls',
    hotlinkEnabled: s.hotlink.enabled,
    hotlinkDomains: s.hotlink.allowed_domains.join('\n'),
    hotlinkAllowEmpty: s.hotlink.allow_empty_referer,
    twEnabled: tw?.enabled ?? false,
    twText: tw?.text ?? '',
    twPos: tw?.position || 'br',
    twOpacity: tw?.opacity != null && tw.opacity >= 0.05 ? tw.opacity : 0.35,
    twSizeRatio: tw?.size_ratio != null && tw.size_ratio >= 0.01 ? tw.size_ratio : 0.05,
    maxEdge: s.processing?.max_edge ?? 0,
    // omit/null → default on (privacy)
    stripExif: s.processing?.strip_exif !== false,
    ann: s.announcement
      ? {
          enabled: !!s.announcement.enabled,
          text: s.announcement.text ?? '',
          link_url: s.announcement.link_url ?? '',
          link_label: s.announcement.link_label ?? '',
          dismissible: s.announcement.dismissible !== false,
          starts_at: s.announcement.starts_at ?? '',
          ends_at: s.announcement.ends_at ?? '',
        }
      : emptyAnn(),
    footerGroups: s.footer?.groups?.length
      ? s.footer.groups.map((g) => ({
          title: g.title ?? '',
          links: (g.links ?? []).map((l) => ({ label: l.label ?? '', url: l.url ?? '' })),
        }))
      : [],
    htmlHead: s.html_inject?.head ?? '',
    htmlBodyEnd: s.html_inject?.body_end ?? '',
  }
}

export function SettingsPage() {
  const { t } = useT()
  const q = useAdminSettings()
  const update = useUpdateSettings()
  const [form, setForm] = useState<FormState | null>(null)
  const [tab, setTab] = useState<SettingsTab>('basic')
  const [testTo, setTestTo] = useState('')
  const [testMsg, setTestMsg] = useState<string | null>(null)
  const testSMTP = useTestSMTP()
  const testModeration = useTestModeration()
  const lexiconFileRef = useRef<HTMLInputElement>(null)
  const lexiconImportMode = useRef<'replace' | 'merge'>('merge')

  // 仅首次加载时初始化;后台 refetch 不打断编辑。保存成功另行用返回值重置(见 submit)。
  useEffect(() => {
    setForm((f) => (f === null && q.data ? formOf(q.data) : f))
  }, [q.data])

  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => setForm((f) => (f ? { ...f, [k]: v } : f))

  const keywordCount = form ? parseLexiconText(form.ocrKeywords).length : 0

  const importLexiconFile = (file: File, mode: 'replace' | 'merge') => {
    const reader = new FileReader()
    reader.onload = () => {
      const text = String(reader.result ?? '')
      setForm((f) => {
        if (!f) return f
        const next = mode === 'merge' ? mergeLexiconText(f.ocrKeywords, text) : parseLexiconText(text).join('\n')
        return { ...f, ocrKeywords: next }
      })
      useGlobal.getState().pushToast(
        mode === 'merge' ? t('adminB.ocrImportMerged') : t('adminB.ocrImportReplaced'),
      )
    }
    reader.onerror = () => useGlobal.getState().pushToast(t('adminB.ocrImportFailed'))
    reader.readAsText(file)
  }

  const exportLexiconFile = () => {
    if (!form) return
    const words = parseLexiconText(form.ocrKeywords)
    const blob = new Blob([formatLexiconExport(words, form.siteName.trim() || undefined)], {
      type: 'text/plain;charset=utf-8',
    })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `imgli-lexicon-${new Date().toISOString().slice(0, 10)}.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  const doTest = () => {
    if (!testTo.includes('@')) return setTestMsg(t('adminB.invalidEmail'))
    setTestMsg(null)
    testSMTP.mutate(
      { to: testTo.trim() },
      {
        onSuccess: () => setTestMsg(t('adminB.sentCheckInbox')),
        onError: (err) =>
          setTestMsg(err instanceof ApiError ? errorText(err.code, err.message) : t('adminB.sendFailed')),
      },
    )
  }

  const doTestModeration = () => {
    testModeration.mutate(undefined, {
      onSuccess: (data) => {
        useGlobal.getState().pushToast(t('adminB.testDoneScore', { score: data.score.toFixed(2) }))
      },
      onError: (err) => {
        useGlobal
          .getState()
          .pushToast(err instanceof ApiError ? errorText(err.code, err.message) : t('adminB.testFailed'))
      },
    })
  }

  const submit = () => {
    if (!form) return
    // 掩码凭据(****开头)仅在路由身份未变时后端才会保留;切 provider/改指向后
    // 发掩码值会被后端「改指向即失效」拒为 400,故此时清空——让后端按「缺凭据」
    // 给可操作校验错,而非阻断正常切换(codex 终审)。
    const orig = q.data?.moderation
    const apiKeyCtxSame = !!orig && form.modProvider === orig.provider && form.modEndpoint.trim() === orig.endpoint
    const akSecretCtxSame =
      !!orig && form.modProvider === orig.provider && form.modRegion === orig.region && form.modAKID === orig.access_key_id
    const ocrKeyCtxSame = !!orig && form.ocrEndpoint.trim() === (orig.ocr_keywords?.endpoint ?? '')
    const outApiKey = form.modApiKey.startsWith('****') && !apiKeyCtxSame ? '' : form.modApiKey
    const outAKSecret = form.modAKSecret.startsWith('****') && !akSecretCtxSame ? '' : form.modAKSecret
    const outOcrKey = form.ocrApiKey.startsWith('****') && !ocrKeyCtxSame ? '' : form.ocrApiKey
    const ocrKeywords = parseLexiconText(form.ocrKeywords)
    update.mutate(
      {
        site_name: form.siteName.trim(),
        registration_mode: form.regMode,
        guest_upload_enabled: form.guestUpload,
        plaza_enabled: form.plazaEnabled,
        moderation: {
          enabled: form.modEnabled,
          provider: form.modProvider,
          endpoint: form.modEndpoint.trim(),
          api_key: outApiKey,
          access_key_id: form.modAKID,
          access_key_secret: outAKSecret,
          region: form.modRegion,
          threshold: form.modThreshold,
          action: form.modAction,
          login_sample_rate: form.loginSampleRate,
          on_plugin_error: form.onPluginError,
          notify_on_reject: form.notifyOnReject,
          ocr_keywords: {
            enabled: form.ocrEnabled,
            endpoint: form.ocrEndpoint.trim(),
            api_key: outOcrKey,
            keywords: ocrKeywords,
            on_hit: form.ocrOnHit,
          },
        },
        smtp: {
          host: form.smtpHost.trim(),
          port: form.smtpPort,
          username: form.smtpUser,
          password: form.smtpPassword,
          from: form.smtpFrom.trim(),
          encryption: form.smtpEnc,
        },
        hotlink: {
          enabled: form.hotlinkEnabled,
          allowed_domains: form.hotlinkDomains.split('\n').map((d) => d.trim()).filter(Boolean),
          allow_empty_referer: form.hotlinkAllowEmpty,
        },
        processing: {
          text_watermark: {
            enabled: form.twEnabled,
            text: form.twText.trim(),
            position: form.twPos,
            opacity: form.twOpacity,
            size_ratio: form.twSizeRatio,
          },
          max_edge: form.maxEdge,
          strip_exif: form.stripExif,
        },
        announcement: {
          enabled: form.ann.enabled,
          text: form.ann.text.trim(),
          link_url: form.ann.link_url.trim(),
          link_label: form.ann.link_label.trim(),
          dismissible: form.ann.dismissible,
          starts_at: form.ann.starts_at.trim(),
          ends_at: form.ann.ends_at.trim(),
        },
        footer: {
          groups: form.footerGroups.map((g) => ({
            title: g.title.trim(),
            links: g.links
              .map((l) => ({ label: l.label.trim(), url: l.url.trim() }))
              .filter((l) => l.label && l.url),
          })),
        },
        html_inject: {
          head: form.htmlHead,
          body_end: form.htmlBodyEnd,
        },
      },
      {
        onSuccess: (data) => {
          setForm(formOf(data))
          useGlobal.getState().pushToast(t('common.saved'))
        },
      },
    )
  }

  const setAnn = <K extends keyof SiteAnnouncement>(k: K, v: SiteAnnouncement[K]) =>
    setForm((f) => (f ? { ...f, ann: { ...f.ann, [k]: v } } : f))

  const patchFooterGroup = (gi: number, patch: Partial<FooterGroup>) =>
    setForm((f) => {
      if (!f) return f
      const groups = f.footerGroups.map((g, i) => (i === gi ? { ...g, ...patch } : g))
      return { ...f, footerGroups: groups }
    })

  const patchFooterLink = (gi: number, li: number, patch: { label?: string; url?: string }) =>
    setForm((f) => {
      if (!f) return f
      const groups = f.footerGroups.map((g, i) => {
        if (i !== gi) return g
        const links = g.links.map((l, j) => (j === li ? { ...l, ...patch } : l))
        return { ...g, links }
      })
      return { ...f, footerGroups: groups }
    })

  return (
    <div>
      <PageHeader kicker="SYSTEM SETTINGS" title={t('adminB.settingsTitle')} />
      {q.isError ? (
        <AdminError onRetry={() => q.refetch()} />
      ) : !form ? (
        <Skeleton height={220} />
      ) : (
        <div className={styles.form}>
          <nav className={styles.tabs} aria-label={t('adminB.settingsTitle')}>
            {SETTINGS_TABS.map((item) => (
              <button
                key={item.key}
                type="button"
                className={[styles.tab, tab === item.key && styles.tabActive].filter(Boolean).join(' ')}
                aria-pressed={tab === item.key}
                onClick={() => setTab(item.key)}
              >
                {t(`adminB.${item.labelKey}`)}
              </button>
            ))}
          </nav>

          {tab === 'basic' && (
            <section className={styles.section}>
              <h2 className={styles.h2}>{t('adminB.basic')}</h2>
              <Input label={t('adminB.siteName')} value={form.siteName} maxLength={64} onChange={(e) => set('siteName', e.target.value)} />
              <div className={styles.field}>
                <span className={styles.label}>{t('adminB.regMode')}</span>
                <Segmented
                  options={[
                    { value: 'open', label: t('adminB.regOpen') },
                    { value: 'invite', label: t('adminB.regInvite') },
                    { value: 'closed', label: t('adminB.regClosed') },
                  ]}
                  value={form.regMode}
                  onChange={(v) => set('regMode', v)}
                />
              </div>
              <div className={styles.field}>
                <div className={styles.sliderHead}>
                  <span className={styles.label}>{t('adminB.guestUpload')}</span>
                  <Toggle aria-label={t('adminB.guestUpload')} checked={form.guestUpload} onChange={(v) => set('guestUpload', v)} />
                </div>
                <span className={styles.hint}>{t('adminB.guestUploadHint')}</span>
              </div>
              <div className={styles.field}>
                <div className={styles.sliderHead}>
                  <span className={styles.label}>{t('adminB.plazaEnabled')}</span>
                  <Toggle aria-label={t('adminB.plazaEnabled')} checked={form.plazaEnabled} onChange={(v) => set('plazaEnabled', v)} />
                </div>
                <span className={styles.hint}>{t('adminB.plazaEnabledHint')}</span>
              </div>
            </section>
          )}

          {tab === 'slots' && (
            <>
              <section className={styles.section}>
                <div className={styles.h2Row}>
                  <h2 className={styles.h2}>{t('adminB.announcement')}</h2>
                  <Toggle checked={form.ann.enabled} onChange={(v) => setAnn('enabled', v)} />
                </div>
                <span className={styles.hint}>{t('adminB.announcementHint')}</span>
                <Input
                  label={t('adminB.announcementText')}
                  value={form.ann.text}
                  maxLength={500}
                  onChange={(e) => setAnn('text', e.target.value)}
                />
                <Input
                  label={t('adminB.announcementLinkUrl')}
                  value={form.ann.link_url}
                  placeholder="https://… or /path"
                  onChange={(e) => setAnn('link_url', e.target.value)}
                />
                <Input
                  label={t('adminB.announcementLinkLabel')}
                  value={form.ann.link_label}
                  maxLength={80}
                  onChange={(e) => setAnn('link_label', e.target.value)}
                />
                <div className={styles.field}>
                  <div className={styles.sliderHead}>
                    <span className={styles.label}>{t('adminB.announcementDismissible')}</span>
                    <Toggle
                      aria-label={t('adminB.announcementDismissible')}
                      checked={form.ann.dismissible}
                      onChange={(v) => setAnn('dismissible', v)}
                    />
                  </div>
                </div>
                <Input
                  label={t('adminB.announcementStarts')}
                  value={form.ann.starts_at}
                  placeholder="2026-07-01T00:00:00Z"
                  onChange={(e) => setAnn('starts_at', e.target.value)}
                />
                <Input
                  label={t('adminB.announcementEnds')}
                  value={form.ann.ends_at}
                  placeholder="2026-08-01T00:00:00Z"
                  onChange={(e) => setAnn('ends_at', e.target.value)}
                />
              </section>

              <section className={styles.section}>
                <h2 className={styles.h2}>{t('adminB.footerLinks')}</h2>
                <span className={styles.hint}>{t('adminB.footerLinksHint')}</span>
                {form.footerGroups.map((g, gi) => (
                  <div key={gi} className={styles.slotCard}>
                    <Input
                      label={t('adminB.footerGroupTitle')}
                      value={g.title}
                      maxLength={80}
                      onChange={(e) => patchFooterGroup(gi, { title: e.target.value })}
                    />
                    {g.links.map((l, li) => (
                      <div key={li} className={styles.slotRow}>
                        <Input
                          label={t('adminB.footerLinkLabel')}
                          value={l.label}
                          maxLength={80}
                          onChange={(e) => patchFooterLink(gi, li, { label: e.target.value })}
                        />
                        <Input
                          label={t('adminB.footerLinkUrl')}
                          value={l.url}
                          onChange={(e) => patchFooterLink(gi, li, { url: e.target.value })}
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          onClick={() =>
                            patchFooterGroup(gi, {
                              links: g.links.filter((_, j) => j !== li),
                            })
                          }
                        >
                          {t('adminB.removeLink')}
                        </Button>
                      </div>
                    ))}
                    <div className={styles.slotActions}>
                      <Button
                        type="button"
                        variant="ghost"
                        onClick={() =>
                          patchFooterGroup(gi, {
                            links: [...g.links, { label: '', url: '' }],
                          })
                        }
                      >
                        {t('adminB.addLink')}
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        onClick={() =>
                          setForm((f) =>
                            f ? { ...f, footerGroups: f.footerGroups.filter((_, i) => i !== gi) } : f,
                          )
                        }
                      >
                        {t('adminB.removeGroup')}
                      </Button>
                    </div>
                  </div>
                ))}
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() =>
                    setForm((f) =>
                      f
                        ? {
                            ...f,
                            footerGroups: [...f.footerGroups, { title: '', links: [{ label: '', url: '' }] }],
                          }
                        : f,
                    )
                  }
                >
                  {t('adminB.addGroup')}
                </Button>
              </section>

              <section className={styles.section}>
                <h2 className={styles.h2}>{t('adminB.htmlInject')}</h2>
                <span className={styles.hintWarn}>{t('adminB.htmlInjectWarn')}</span>
                <div className={styles.field}>
                  <span className={styles.label}>{t('adminB.htmlHead')}</span>
                  <textarea
                    className={styles.textarea}
                    rows={5}
                    value={form.htmlHead}
                    onChange={(e) => set('htmlHead', e.target.value)}
                    spellCheck={false}
                  />
                </div>
                <div className={styles.field}>
                  <span className={styles.label}>{t('adminB.htmlBodyEnd')}</span>
                  <textarea
                    className={styles.textarea}
                    rows={5}
                    value={form.htmlBodyEnd}
                    onChange={(e) => set('htmlBodyEnd', e.target.value)}
                    spellCheck={false}
                  />
                </div>
              </section>
            </>
          )}

          {tab === 'moderation' && (
            <section className={styles.section}>
              <div className={styles.h2Row}>
                <h2 className={styles.h2}>{t('adminB.moderation')}</h2>
                <Toggle checked={form.modEnabled} onChange={(v) => set('modEnabled', v)} />
              </div>
              <div className={styles.field}>
                <span className={styles.label}>{t('adminB.provider')}</span>
                <Segmented
                  options={[
                    { value: 'webhook', label: 'Webhook' },
                    { value: 'aliyun', label: t('adminB.providerAliyun') },
                    { value: 'tencent', label: t('adminB.providerTencent') },
                    { value: 'openai', label: 'OpenAI' },
                    { value: 'nsfwjs', label: 'NSFWJS' },
                  ]}
                  value={form.modProvider}
                  onChange={(v) => set('modProvider', v)}
                />
              </div>
              {(form.modProvider === 'webhook' || form.modProvider === 'nsfwjs') && (
                <Input label={t('adminB.webhookUrl')} placeholder="https://..." value={form.modEndpoint} onChange={(e) => set('modEndpoint', e.target.value)} />
              )}
              {(form.modProvider === 'webhook' || form.modProvider === 'nsfwjs' || form.modProvider === 'openai') && (
                <Input
                  label="API Key"
                  placeholder={t('adminB.noKeyPlaceholder')}
                  value={form.modApiKey}
                  extra={<span className={styles.hint}>{t('adminB.secretMaskHintSettings')}</span>}
                  onChange={(e) => set('modApiKey', e.target.value)}
                  onFocus={(e) => e.target.select()}
                />
              )}
              {(form.modProvider === 'aliyun' || form.modProvider === 'tencent') && (
                <>
                  <Input label="AccessKey ID" placeholder="AccessKey ID" value={form.modAKID} onChange={(e) => set('modAKID', e.target.value)} />
                  <Input
                    label="AccessKey Secret"
                    placeholder={t('adminB.noKeyPlaceholder')}
                    value={form.modAKSecret}
                    extra={<span className={styles.hint}>{t('adminB.secretMaskHintSettings')}</span>}
                    onChange={(e) => set('modAKSecret', e.target.value)}
                    onFocus={(e) => e.target.select()}
                  />
                  <Input
                    label="Region"
                    placeholder={form.modProvider === 'aliyun' ? 'cn-shanghai' : 'ap-guangzhou'}
                    value={form.modRegion}
                    onChange={(e) => set('modRegion', e.target.value)}
                  />
                </>
              )}
              <div className={styles.field}>
                <div className={styles.sliderHead}>
                  <span className={styles.label}>{t('adminB.threshold')}</span>
                  <span className={styles.mono}>{form.modThreshold.toFixed(2)}</span>
                </div>
                <input
                  className={styles.slider}
                  type="range"
                  min={0}
                  max={1}
                  step={0.01}
                  value={form.modThreshold}
                  aria-label={t('adminB.threshold')}
                  onChange={(e) => set('modThreshold', Number(e.target.value))}
                />
              </div>
              <div className={styles.field}>
                <span className={styles.label}>{t('adminB.overThresholdAction')}</span>
                <Segmented
                  options={[
                    { value: 'pending', label: t('adminB.actionPending') },
                    { value: 'rejected', label: t('adminB.actionRejected') },
                  ]}
                  value={form.modAction}
                  onChange={(v) => set('modAction', v)}
                />
              </div>
              <div className={styles.field}>
                <div className={styles.sliderHead}>
                  <span className={styles.label}>{t('adminB.loginSampleRate')}</span>
                  <span className={styles.mono}>{(form.loginSampleRate * 100).toFixed(0)}%</span>
                </div>
                <input
                  className={styles.slider}
                  type="range"
                  min={0}
                  max={1}
                  step={0.05}
                  value={form.loginSampleRate}
                  aria-label={t('adminB.loginSampleRate')}
                  onChange={(e) => set('loginSampleRate', Number(e.target.value))}
                />
                <span className={styles.hint}>{t('adminB.loginSampleRateHint')}</span>
              </div>
              <div className={styles.field}>
                <span className={styles.label}>{t('adminB.onPluginError')}</span>
                <Segmented
                  options={[
                    { value: 'open', label: t('adminB.onPluginErrorOpen') },
                    { value: 'review', label: t('adminB.onPluginErrorReview') },
                  ]}
                  value={form.onPluginError}
                  onChange={(v) => set('onPluginError', v)}
                />
                <span className={styles.hint}>{t('adminB.onPluginErrorHint')}</span>
              </div>
              <div className={styles.row}>
                <span className={styles.label}>{t('adminB.notifyOnReject')}</span>
                <Toggle
                  aria-label={t('adminB.notifyOnReject')}
                  checked={form.notifyOnReject}
                  onChange={(v) => set('notifyOnReject', v)}
                />
              </div>
              <span className={styles.hint}>{t('adminB.notifyOnRejectHint')}</span>
              <div className={styles.field}>
                <Button variant="secondary" disabled={testModeration.isPending} onClick={doTestModeration}>
                  {t('adminB.testModeration')}
                </Button>
              </div>
            </section>
          )}

          {tab === 'ocr' && (
            <section className={styles.section}>
              <div className={styles.h2Row}>
                <h2 className={styles.h2}>{t('adminB.ocrSection')}</h2>
                <Toggle
                  aria-label={t('adminB.ocrEnable')}
                  checked={form.ocrEnabled}
                  onChange={(v) => set('ocrEnabled', v)}
                />
              </div>
              <span className={styles.hint}>{t('adminB.ocrEnableHint')}</span>
              <Input
                label={t('adminB.ocrEndpoint')}
                placeholder={t('adminB.ocrEndpointPlaceholder')}
                value={form.ocrEndpoint}
                onChange={(e) => set('ocrEndpoint', e.target.value)}
              />
              <Input
                label={t('adminB.ocrApiKey')}
                placeholder={t('adminB.noKeyPlaceholder')}
                value={form.ocrApiKey}
                extra={<span className={styles.hint}>{t('adminB.secretMaskHintSettings')}</span>}
                onChange={(e) => set('ocrApiKey', e.target.value)}
                onFocus={(e) => e.target.select()}
              />
              <div className={styles.field}>
                <div className={styles.sliderHead}>
                  <label className={styles.label} htmlFor="ocr-keywords">
                    {t('adminB.ocrKeywords')}
                  </label>
                  <span className={styles.mono}>{t('adminB.ocrKeywordCount', { count: keywordCount })}</span>
                </div>
                <textarea
                  id="ocr-keywords"
                  className={styles.textarea}
                  rows={10}
                  placeholder={t('adminB.ocrKeywordsPlaceholder')}
                  value={form.ocrKeywords}
                  onChange={(e) => set('ocrKeywords', e.target.value)}
                />
                <div className={styles.lexiconToolbar}>
                  <input
                    ref={lexiconFileRef}
                    type="file"
                    accept=".txt,text/plain"
                    className={styles.fileInput}
                    aria-hidden
                    tabIndex={-1}
                    onChange={(e) => {
                      const f = e.target.files?.[0]
                      e.target.value = ''
                      if (f) importLexiconFile(f, lexiconImportMode.current)
                    }}
                  />
                  <Button
                    variant="secondary"
                    type="button"
                    onClick={() => {
                      lexiconImportMode.current = 'merge'
                      lexiconFileRef.current?.click()
                    }}
                  >
                    {t('adminB.ocrImportMerge')}
                  </Button>
                  <Button
                    variant="secondary"
                    type="button"
                    onClick={() => {
                      lexiconImportMode.current = 'replace'
                      lexiconFileRef.current?.click()
                    }}
                  >
                    {t('adminB.ocrImportReplace')}
                  </Button>
                  <Button variant="secondary" type="button" onClick={exportLexiconFile} disabled={keywordCount === 0}>
                    {t('adminB.ocrExport')}
                  </Button>
                </div>
                <span className={styles.hint}>{t('adminB.ocrKeywordsHint')}</span>
                <span className={styles.hint}>{t('adminB.ocrLexiconNote')}</span>
              </div>
              <div className={styles.field}>
                <span className={styles.label}>{t('adminB.ocrOnHit')}</span>
                <Segmented
                  options={[
                    { value: 'review', label: t('adminB.actionPending') },
                    { value: 'block', label: t('adminB.actionRejected') },
                  ]}
                  value={form.ocrOnHit}
                  onChange={(v) => set('ocrOnHit', v)}
                />
              </div>
            </section>
          )}

          {tab === 'smtp' && (
            <section className={styles.section}>
              <h2 className={styles.h2}>{t('adminB.smtpSection')}</h2>
              <Input label={t('adminB.smtpHost')} placeholder={t('adminB.smtpHostPlaceholder')} value={form.smtpHost} onChange={(e) => set('smtpHost', e.target.value)} />
              <Input label={t('adminB.port')} type="number" value={String(form.smtpPort)} onChange={(e) => set('smtpPort', Number(e.target.value) || 0)} />
              <Input label={t('adminB.username')} placeholder={t('adminB.noAuthPlaceholder')} value={form.smtpUser} onChange={(e) => set('smtpUser', e.target.value)} />
              <Input
                label={t('adminB.smtpPassword')}
                placeholder={t('adminB.noPasswordPlaceholder')}
                value={form.smtpPassword}
                extra={<span className={styles.hint}>{t('adminB.passwordMaskHint')}</span>}
                onChange={(e) => set('smtpPassword', e.target.value)}
                onFocus={(e) => e.target.select()}
              />
              <Input label={t('adminB.from')} placeholder="no-reply@img.li" value={form.smtpFrom} onChange={(e) => set('smtpFrom', e.target.value)} />
              <div className={styles.field}>
                <span className={styles.label}>{t('adminB.encryption')}</span>
                <Segmented
                  options={[
                    { value: 'none', label: t('adminB.noEncryption') },
                    { value: 'starttls', label: 'STARTTLS' },
                    { value: 'ssl', label: 'SSL' },
                  ]}
                  value={form.smtpEnc}
                  onChange={(v) => set('smtpEnc', v)}
                />
              </div>
              <div className={styles.field}>
                <span className={styles.label}>{t('adminB.testSend')}</span>
                <div className={styles.testRow}>
                  <Input label={t('adminB.testRecipient')} placeholder="you@example.com" value={testTo} onChange={(e) => setTestTo(e.target.value)} />
                  <Button variant="secondary" disabled={testSMTP.isPending} onClick={doTest}>{t('adminB.sendTestEmail')}</Button>
                </div>
                {testMsg && <span className={styles.hint}>{testMsg}</span>}
              </div>
            </section>
          )}

          {tab === 'hotlink' && (
            <section className={styles.section}>
              <div className={styles.h2Row}>
                <h2 className={styles.h2}>{t('adminB.hotlink')}</h2>
                <Toggle aria-label={t('adminB.enableHotlink')} checked={form.hotlinkEnabled} onChange={(v) => set('hotlinkEnabled', v)} />
              </div>
              <div className={styles.field}>
                <label className={styles.label} htmlFor="hotlink-domains">{t('adminB.allowedDomains')}</label>
                <textarea
                  id="hotlink-domains"
                  className={styles.textarea}
                  rows={4}
                  placeholder={'example.com\n*.cdn.example.com'}
                  value={form.hotlinkDomains}
                  onChange={(e) => set('hotlinkDomains', e.target.value)}
                />
                <span className={styles.hint}>{t('adminB.domainsHint')}</span>
              </div>
              <div className={styles.field}>
                <div className={styles.sliderHead}>
                  <span className={styles.label}>{t('adminB.allowEmptyReferer')}</span>
                  <Toggle aria-label={t('adminB.allowEmptyReferer')} checked={form.hotlinkAllowEmpty} onChange={(v) => set('hotlinkAllowEmpty', v)} />
                </div>
                <span className={styles.hint}>{t('adminB.emptyRefererHint')}</span>
              </div>
            </section>
          )}

          {tab === 'processing' && (
            <section className={styles.section}>
              <div className={styles.h2Row}>
                <h2 className={styles.h2}>{t('adminB.processing')}</h2>
              </div>
              <div className={styles.field}>
                <div className={styles.sliderHead}>
                  <span className={styles.label}>{t('adminB.stripExif')}</span>
                  <Toggle aria-label={t('adminB.stripExif')} checked={form.stripExif} onChange={(v) => set('stripExif', v)} />
                </div>
                <span className={styles.hint}>{t('adminB.stripExifHint')}</span>
              </div>
              <div className={styles.h2Row}>
                <h3 className={styles.h2}>{t('adminB.textWatermark')}</h3>
                <Toggle aria-label={t('adminB.enableTextWatermark')} checked={form.twEnabled} onChange={(v) => set('twEnabled', v)} />
              </div>
              <Input
                label={t('adminB.textWatermark')}
                placeholder={t('adminB.textWatermarkPlaceholder')}
                value={form.twText}
                onChange={(e) => set('twText', e.target.value)}
              />
              <div className={styles.field}>
                <label className={styles.label} htmlFor="proc-wm-pos">
                  {t('adminB.watermarkPos')}
                </label>
                <select
                  id="proc-wm-pos"
                  className={styles.select}
                  value={form.twPos}
                  onChange={(e) => set('twPos', e.target.value)}
                >
                  {POSITIONS.map((pos) => (
                    <option key={pos.value} value={pos.value}>
                      {t(pos.label)}
                    </option>
                  ))}
                </select>
              </div>
              <div className={styles.field}>
                <div className={styles.sliderHead}>
                  <span className={styles.label}>{t('adminB.opacity')}</span>
                  <span className={styles.mono}>{form.twOpacity.toFixed(2)}</span>
                </div>
                <input
                  className={styles.slider}
                  type="range"
                  min={0.05}
                  max={1}
                  step={0.05}
                  value={form.twOpacity}
                  aria-label={t('adminB.textOpacityAria')}
                  onChange={(e) => set('twOpacity', Number(e.target.value))}
                />
              </div>
              <div className={styles.field}>
                <div className={styles.sliderHead}>
                  <span className={styles.label}>{t('adminB.sizeRatio')}</span>
                  <span className={styles.mono}>{form.twSizeRatio.toFixed(2)}</span>
                </div>
                <input
                  className={styles.slider}
                  type="range"
                  min={0.01}
                  max={0.2}
                  step={0.01}
                  value={form.twSizeRatio}
                  aria-label={t('adminB.sizeRatio')}
                  onChange={(e) => set('twSizeRatio', Number(e.target.value))}
                />
              </div>
              <Input
                label={t('adminB.maxEdge')}
                type="number"
                min={0}
                max={16384}
                value={String(form.maxEdge)}
                onChange={(e) => set('maxEdge', Number(e.target.value) || 0)}
                extra={<span className={styles.hint}>{t('adminB.maxEdgeHint')}</span>}
              />
            </section>
          )}

          <div className={styles.actions}>
            <Button variant="primary" disabled={update.isPending} onClick={submit}>
              {t('adminB.saveSettings')}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
