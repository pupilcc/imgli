import type { AdminSettings, ShareBranding, SiteAnnouncement } from '../../../api/types'
import { toLocaleMap } from '../../../lib/locale'

export type FormLocale = { zh: string; en: string }

export type FormFooterLink = { label: FormLocale; url: string }
export type FormFooterGroup = { title: FormLocale; links: FormFooterLink[] }

export type ModProvider = 'webhook' | 'aliyun' | 'tencent' | 'openai' | 'nsfwjs'

export type SettingsTab = 'basic' | 'slots' | 'moderation' | 'ocr' | 'smtp' | 'hotlink' | 'processing'

export const SETTINGS_TABS: {
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

export const emptyLocale = (): FormLocale => ({ zh: '', en: '' })

export const emptyAnn = (): SiteAnnouncement => ({
  enabled: false,
  text: emptyLocale(),
  link_url: '',
  link_label: emptyLocale(),
  dismissible: true,
  starts_at: '',
  ends_at: '',
})

export interface FormState {
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
  footerGroups: FormFooterGroup[]
  htmlHead: string
  htmlBodyEnd: string
  helpUrl: string
  upgradeUrl: string
  registerNotice: FormLocale
  shareBranding: ShareBranding
  faviconUrl: string
  sourceUrl: string
  ossCredit: 'on' | 'off'
  aboutEnabled: boolean
  aboutBody: FormLocale
  welcomeEmail: boolean
}

export type FormSet = <K extends keyof FormState>(k: K, v: FormState[K]) => void

const MOD_PROVIDERS: ModProvider[] = ['webhook', 'aliyun', 'tencent', 'openai', 'nsfwjs']

export function formOf(s: AdminSettings): FormState {
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
    stripExif: s.processing?.strip_exif !== false,
    ann: s.announcement
      ? {
          enabled: !!s.announcement.enabled,
          text: toLocaleMap(s.announcement.text),
          link_url: s.announcement.link_url ?? '',
          link_label: toLocaleMap(s.announcement.link_label),
          dismissible: s.announcement.dismissible !== false,
          starts_at: s.announcement.starts_at ?? '',
          ends_at: s.announcement.ends_at ?? '',
        }
      : emptyAnn(),
    footerGroups: s.footer?.groups?.length
      ? s.footer.groups.map((g) => ({
          title: toLocaleMap(g.title),
          links: (g.links ?? []).map((l) => ({
            label: toLocaleMap(l.label),
            url: l.url ?? '',
          })),
        }))
      : [],
    htmlHead: s.html_inject?.head ?? '',
    htmlBodyEnd: s.html_inject?.body_end ?? '',
    helpUrl: s.help_url ?? '',
    upgradeUrl: s.upgrade_url ?? '',
    registerNotice: toLocaleMap(s.register_notice),
    shareBranding:
      s.share_branding === 'off' || s.share_branding === 'links'
        ? s.share_branding
        : 'site',
    faviconUrl: s.favicon_url ?? '',
    sourceUrl: s.source_url ?? '',
    ossCredit: s.oss_credit === 'off' ? 'off' : 'on',
    aboutEnabled: !!s.about_enabled,
    aboutBody: toLocaleMap(s.about_body),
    welcomeEmail: s.welcome_email !== false,
  }
}
