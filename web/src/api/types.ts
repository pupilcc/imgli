export interface Preferences {
  default_album_id: number | null
  default_visibility: '' | 'public' | 'private'
  default_policy_id: number | null
  auto_copy_format: '' | 'url' | 'markdown' | 'html' | 'bbcode'
  watermark: {
    enabled: boolean
    position: string
    opacity: number
    margin: number
  }
  /** 用户语言偏好;空串=跟随前端 detect */
  lang?: 'zh' | 'en' | ''
}

/** 九宫格水印位置(用户图水印 / 站级文字水印共用)。label 为 i18n key，渲染时 t(label)。 */
export const POSITIONS = [
  { value: 'tl', label: 'settings.wmPos.tl' },
  { value: 'tc', label: 'settings.wmPos.tc' },
  { value: 'tr', label: 'settings.wmPos.tr' },
  { value: 'ml', label: 'settings.wmPos.ml' },
  { value: 'mc', label: 'settings.wmPos.mc' },
  { value: 'mr', label: 'settings.wmPos.mr' },
  { value: 'bl', label: 'settings.wmPos.bl' },
  { value: 'bc', label: 'settings.wmPos.bc' },
  { value: 'br', label: 'settings.wmPos.br' },
] as const

export interface PolicyOption {
  id: number
  name: string
}

export interface User {
  id: number
  username: string
  email: string
  nickname: string
  is_admin: boolean
  email_verified: boolean
  created_at: string
  preferences: Preferences
  avatar_url: string
  watermark_set: boolean
  public_profile: boolean
}

export interface Quota {
  used: number
  total: number
  max_file_size: number
  allowed_exts: string[]
}

export interface Links {
  url: string
  markdown: string
  html: string
  bbcode: string
  thumbnail_url: string
}

export interface UploadResult {
  key: string
  name: string
  size: number
  instant: boolean
  links: Links
  /** RFC3339 or null */
  expires_at?: string | null
}

export interface Album {
  id: number
  name: string
  visibility: string
  image_count: number
  cover_key: string
  created_at: string
}

export interface ImageItem {
  key: string
  /** vanity 别名，可选 */
  slug?: string | null
  name: string
  ext: string
  size: number
  width: number
  height: number
  visibility: string
  album_id: number | null
  created_at: string
  /** RFC3339;null=永久 */
  expires_at: string | null
  links: Links
}

export interface ImageDetail extends ImageItem {
  mime: string
  upload_ip: string
}

export interface ImageStats {
  total: number
  daily: { date: string; views: number }[]
}

export interface BatchResult {
  key: string
  ok: boolean
  error?: string
}

export interface ImagesPage {
  items: ImageItem[]
  next_cursor: string
}

export interface TrashItem {
  key: string
  name: string
  ext: string
  size: number
  width: number
  height: number
  deleted_at: string
  days_left: number
}

export interface TrashPage {
  items: TrashItem[]
  next_cursor: string
}

export interface ApiToken {
  id: number
  name: string
  scope: 'upload' | 'full'
  created_at: string
  last_used_at: string | null
  token?: string
}

export interface DailyCount {
  date: string
  count: number
}

export interface TrafficDay {
  date: string
  views: number
}

export interface TopReferer {
  host: string
  count: number
}

export interface AdminStats {
  users: number
  images: number
  storage: number
  today_uploads: number
  pending_images: number
  rejected_images: number
  tasks_pending: number
  tasks_running: number
  daily: DailyCount[] | null
  traffic_7d: TrafficDay[]
  top_referers: TopReferer[]
}

export interface AuditLog {
  id: number
  actor_id: number | null
  actor_type: string
  action: string
  detail: string
  ip: string
  created_at: string
}

export interface AdminLogsPage {
  items: AuditLog[]
  total: number
  page: number
  limit: number
}

export interface AdminUser {
  id: number
  username: string
  email: string
  nickname: string
  group_id: number
  status: string
  is_admin: boolean
  used_storage: number
  created_at: string
  image_count: number
  email_verified?: boolean
}

export interface AdminUsersPage {
  items: AdminUser[]
  total: number
  page: number
  limit: number
}

/** 机审触发摘要（审核队列；来自 moderation_flag audit.results） */
export interface ModerationTrigger {
  plugin: string
  severity: string
  score?: number | null
  hits?: string[]
}

export interface AdminImageItem {
  key: string
  name: string
  ext: string
  size: number
  visibility: string
  status: string
  is_whitelisted: boolean
  nsfw_score: number | null
  username: string
  user_id: number
  created_at: string
  links: Links
  /** 仅审核队列可选返回 */
  triggers?: ModerationTrigger[]
}

export interface AdminImagesPage {
  items: AdminImageItem[]
  total: number
  page: number
  limit: number
}

export interface ReviewBatchResult {
  key: string
  ok: boolean
  error?: string
}

export interface AdminGroup {
  id: number
  name: string
  is_default: boolean
  is_guest: boolean
  storage_quota: number
  max_file_size: number
  rate_per_minute: number
  rate_per_hour: number
  rate_per_day: number
  allowed_exts: string[]
  allowed_policy_ids: number[] | null
  created_at: string
  user_count: number
}

export interface AdminPolicy {
  id: number
  name: string
  driver: string
  config: string
  cdn_domain: string
  path_template: string
  enabled: boolean
  created_at: string
  file_count: number
  used_bytes: number
}

export interface AdminOCRKeywords {
  enabled: boolean
  endpoint: string
  api_key: string
  keywords: string[] | null
  on_hit: string
}

export interface AdminModeration {
  enabled: boolean
  provider: 'webhook' | 'aliyun' | 'tencent' | 'openai' | 'nsfwjs'
  endpoint: string
  api_key: string
  access_key_id: string
  access_key_secret: string
  region: string
  threshold: number
  action: string
  /** OCR+词表插件；缺省或 null 时前端按关闭处理 */
  ocr_keywords?: AdminOCRKeywords | null
  /** 登录用户机审入队概率 0–1；游客恒全审。缺省 1 */
  login_sample_rate?: number
  /** open | review；插件失败策略。缺省 open */
  on_plugin_error?: string
  /** 拒绝后邮件通知属主。缺省 false */
  notify_on_reject?: boolean
}

export interface AdminSMTP {
  host: string
  port: number
  username: string
  password: string
  from: string
  encryption: 'none' | 'starttls' | 'ssl'
}

export interface HotlinkSettings {
  enabled: boolean
  allowed_domains: string[]
  allow_empty_referer: boolean
}

export interface TextWatermarkSettings {
  enabled: boolean
  text: string
  position: string
  opacity: number
  size_ratio: number
}

export interface ProcessingSettings {
  text_watermark: TextWatermarkSettings
  max_edge: number
}

export interface AdminSettings {
  site_name: string
  registration_mode: string
  guest_upload_enabled: boolean
  plaza_enabled: boolean
  moderation: AdminModeration
  smtp: AdminSMTP
  hotlink: HotlinkSettings
  processing: ProcessingSettings
}

export interface GuestLimits {
  max_file_size: number
  allowed_exts: string[]
  per_day: number
}

export interface PublicConfig {
  site_name: string
  registration_mode: string
  guest_upload_enabled: boolean
  guest: GuestLimits | null
  plaza_enabled: boolean
}

export interface DiscoverAuthor {
  user_id: number
  username: string
  nickname: string
  avatar_version: number
}

export interface DiscoverRow {
  key: string
  name: string
  ext: string
  created_at: string
  views: number
  author: DiscoverAuthor
}

export interface DiscoverPage {
  items: DiscoverRow[]
  next_cursor: string
}

export interface PublicProfileData {
  username: string
  nickname: string
  avatar_version: number
  joined_at: string
  public_image_count: number
}

export interface AdminInvite {
  id: number
  code: string
  status: 'unused' | 'used' | 'expired'
  created_by_name: string
  used_by_name: string
  created_at: string
  expires_at: string | null
  used_at: string | null
}

export interface AdminInvitesPage {
  items: AdminInvite[]
  total: number
  page: number
  limit: number
}
