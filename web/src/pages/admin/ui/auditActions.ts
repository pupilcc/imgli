import { t } from '../../../i18n'

/** action 常量 → adminB i18n key（声明序即筛选下拉序）。 */
const ACTION_I18N: Record<string, string> = {
  user_update: 'adminB.actUserUpdate',
  user_ban: 'adminB.actUserBan',
  user_reset_password: 'adminB.actUserResetPassword',
  review_approve: 'adminB.actReviewApprove',
  review_reject: 'adminB.actReviewReject',
  image_admin_delete: 'adminB.actImageAdminDelete',
  image_admin_purge: 'adminB.actImageAdminPurge',
  image_whitelist: 'adminB.actImageWhitelist',
  group_create: 'adminB.actGroupCreate',
  group_update: 'adminB.actGroupUpdate',
  group_delete: 'adminB.actGroupDelete',
  group_lifecycle_apply: 'adminB.actGroupLifecycleApply',
  policy_create: 'adminB.actPolicyCreate',
  policy_update: 'adminB.actPolicyUpdate',
  policy_delete: 'adminB.actPolicyDelete',
  policy_test: 'adminB.actPolicyTest',
  policy_enable_compat: 'adminB.actPolicyEnableCompat',
  settings_update: 'adminB.actSettingsUpdate',
  smtp_test: 'adminB.actSmtpTest',
  moderation_flag: 'adminB.actModerationFlag',
  invite_create: 'adminB.actInviteCreate',
  invite_revoke: 'adminB.actInviteRevoke',
}

export function actionLabel(action: string): string {
  const key = ACTION_I18N[action]
  return key ? t(key) : action
}

/** 运行时解析的 action 标签（兼容 Dashboard 的属性访问）。 */
export const ACTION_LABELS: Record<string, string> = new Proxy({} as Record<string, string>, {
  get(_target, prop: string | symbol) {
    if (typeof prop !== 'string' || !(prop in ACTION_I18N)) return undefined
    return t(ACTION_I18N[prop])
  },
  has(_target, prop: string | symbol) {
    return typeof prop === 'string' && prop in ACTION_I18N
  },
  ownKeys() {
    return Object.keys(ACTION_I18N)
  },
  getOwnPropertyDescriptor(_target, prop) {
    if (typeof prop === 'string' && prop in ACTION_I18N) {
      return { enumerable: true, configurable: true, value: t(ACTION_I18N[prop]) }
    }
    return undefined
  },
})

const RED = new Set(['review_reject', 'image_admin_delete', 'image_admin_purge', 'group_delete', 'policy_delete', 'user_ban', 'invite_revoke'])
const GREEN = new Set(['review_approve', 'group_create', 'policy_create', 'invite_create'])
const WARN = new Set(['moderation_flag'])

export function dotColor(action: string): string {
  if (RED.has(action)) return 'var(--err)'
  if (WARN.has(action)) return 'var(--warn)'
  if (GREEN.has(action)) return 'var(--ok)'
  return 'var(--muted)'
}

/** 操作筛选下拉选项(顺序即 ACTION_I18N 声明序)。 */
export function actionOptions(): { value: string; label: string }[] {
  return Object.keys(ACTION_I18N).map((value) => ({ value, label: t(ACTION_I18N[value]) }))
}

/** @deprecated 模块加载时固化标签；请用 actionOptions()。保留以兼容旧 import。 */
export const ACTION_OPTIONS: { value: string; label: string }[] = Object.keys(ACTION_I18N).map((value) => ({
  value,
  get label() {
    return t(ACTION_I18N[value])
  },
}))

const ACTOR_I18N: Record<string, string> = {
  user: 'adminB.actorUser',
  admin: 'adminB.actorAdmin',
  system: 'adminB.actorSystem',
}

export function actorTypeLabel(type: string): string {
  const key = ACTOR_I18N[type]
  return key ? t(key) : type
}

export const ACTOR_TYPE_LABELS: Record<string, string> = new Proxy({} as Record<string, string>, {
  get(_target, prop: string | symbol) {
    if (typeof prop !== 'string' || !(prop in ACTOR_I18N)) return undefined
    return t(ACTOR_I18N[prop])
  },
  has(_target, prop: string | symbol) {
    return typeof prop === 'string' && prop in ACTOR_I18N
  },
  ownKeys() {
    return Object.keys(ACTOR_I18N)
  },
  getOwnPropertyDescriptor(_target, prop) {
    if (typeof prop === 'string' && prop in ACTOR_I18N) {
      return { enumerable: true, configurable: true, value: t(ACTOR_I18N[prop]) }
    }
    return undefined
  },
})
