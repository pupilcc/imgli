import { useState, useId } from 'react'
import {
  useAdminGroups,
  useAdminPolicies,
  useApplyGroupLifecycle,
  useCreateGroup,
  useDeleteGroup,
  usePreviewGroupLifecycle,
  useUpdateGroup,
  type GroupWriteBody,
} from '../../../api/adminHooks'
import type { AdminGroup } from '../../../api/types'
import { useT } from '../../../i18n'
import { PageHeader } from '../../../shell/PageHeader'
import { useGlobal } from '../../../store'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { InlineConfirm } from '../../../ui/InlineConfirm'
import { Input } from '../../../ui/Input'
import { AdminQueryGate } from '../ui/AdminQueryGate'
import forms from '../ui/adminForms.module.css'
import styles from './GroupsPage.module.css'

function lifecycleBadges(g: AdminGroup, t: (k: string, v?: Record<string, string | number>) => string): string[] {
  const badges: string[] = []
  const maxSec = g.max_expires_in ?? 0
  const force = g.force_max_age_days ?? 0
  const ret = g.retention_days ?? 0
  let capDays = 0
  if (maxSec > 0) capDays = Math.ceil(maxSec / 86400)
  if (force > 0 && (capDays === 0 || force < capDays)) capDays = force
  if (capDays > 0) badges.push(t('adminA.lifecycleBadgeMax', { days: capDays }))
  if (force > 0) badges.push(t('adminA.lifecycleBadgeForce', { days: force }))
  if (ret > 0) badges.push(t('adminA.lifecycleBadgeRetention', { days: ret }))
  return badges
}

const GB = 1024 ** 3
const MB = 1024 ** 2
const toGB = (b: number) => String(+(b / GB).toFixed(2))
const toMB = (b: number) => String(+(b / MB).toFixed(2))

interface FormState {
  name: string
  quotaGB: string
  maxMB: string
  /** 月流量硬顶 GB；0=不限 */
  bwGB: string
  perMin: string
  perHour: string
  perDay: string
  exts: string[]
  policyIds: number[]
  defaultExpires: string
  maxExpires: string
  defaultMaxViews: string
  maxMaxViews: string
  retentionDays: string
  forceMaxAgeDays: string
}

const NEW_FORM: FormState = {
  name: '', quotaGB: '10', maxMB: '20', bwGB: '5', perMin: '20', perHour: '200', perDay: '1000',
  exts: ['png', 'jpg', 'jpeg', 'gif', 'webp'], policyIds: [],
  defaultExpires: '0', maxExpires: '0', defaultMaxViews: '0', maxMaxViews: '0',
  retentionDays: '0', forceMaxAgeDays: '0',
}

function formOf(g: AdminGroup): FormState {
  return {
    name: g.name,
    quotaGB: toGB(g.storage_quota),
    maxMB: toMB(g.max_file_size),
    bwGB: toGB(g.bandwidth_quota_month ?? 0),
    perMin: String(g.rate_per_minute),
    perHour: String(g.rate_per_hour),
    perDay: String(g.rate_per_day),
    exts: g.allowed_exts ?? [],
    policyIds: g.allowed_policy_ids ?? [],
    defaultExpires: String(g.default_expires_in ?? 0),
    maxExpires: String(g.max_expires_in ?? 0),
    defaultMaxViews: String(g.default_max_views ?? 0),
    maxMaxViews: String(g.max_max_views ?? 0),
    retentionDays: String(g.retention_days ?? 0),
    forceMaxAgeDays: String(g.force_max_age_days ?? 0),
  }
}

function createBody(f: FormState): GroupWriteBody {
  return {
    name: f.name.trim(),
    storage_quota: Math.round(Number(f.quotaGB) * GB),
    max_file_size: Math.round(Number(f.maxMB) * MB),
    bandwidth_quota_month: Math.round(Number(f.bwGB) * GB),
    rate_per_minute: Number(f.perMin),
    rate_per_hour: Number(f.perHour),
    rate_per_day: Number(f.perDay),
    allowed_exts: f.exts,
    allowed_policy_ids: f.policyIds,
    default_expires_in: Number(f.defaultExpires) || 0,
    max_expires_in: Number(f.maxExpires) || 0,
    default_max_views: Number(f.defaultMaxViews) || 0,
    max_max_views: Number(f.maxMaxViews) || 0,
    retention_days: Number(f.retentionDays) || 0,
    force_max_age_days: Number(f.forceMaxAgeDays) || 0,
  }
}

// 差异提交:只发相对已加载组变化的字段(数值比显示串,避开 bytes 往返舍入抖动)。
function patchBody(o: AdminGroup, f: FormState): GroupWriteBody {
  const b: GroupWriteBody = {}
  if (f.name.trim() !== o.name) b.name = f.name.trim()
  if (f.quotaGB !== toGB(o.storage_quota)) b.storage_quota = Math.round(Number(f.quotaGB) * GB)
  if (f.maxMB !== toMB(o.max_file_size)) b.max_file_size = Math.round(Number(f.maxMB) * MB)
  if (f.bwGB !== toGB(o.bandwidth_quota_month ?? 0)) b.bandwidth_quota_month = Math.round(Number(f.bwGB) * GB)
  if (Number(f.perMin) !== o.rate_per_minute) b.rate_per_minute = Number(f.perMin)
  if (Number(f.perHour) !== o.rate_per_hour) b.rate_per_hour = Number(f.perHour)
  if (Number(f.perDay) !== o.rate_per_day) b.rate_per_day = Number(f.perDay)
  if (JSON.stringify(f.exts) !== JSON.stringify(o.allowed_exts ?? [])) b.allowed_exts = f.exts
  if (JSON.stringify(f.policyIds) !== JSON.stringify(o.allowed_policy_ids ?? [])) b.allowed_policy_ids = f.policyIds
  if (Number(f.defaultExpires) !== (o.default_expires_in ?? 0)) b.default_expires_in = Number(f.defaultExpires) || 0
  if (Number(f.maxExpires) !== (o.max_expires_in ?? 0)) b.max_expires_in = Number(f.maxExpires) || 0
  if (Number(f.defaultMaxViews) !== (o.default_max_views ?? 0)) b.default_max_views = Number(f.defaultMaxViews) || 0
  if (Number(f.maxMaxViews) !== (o.max_max_views ?? 0)) b.max_max_views = Number(f.maxMaxViews) || 0
  if (Number(f.retentionDays) !== (o.retention_days ?? 0)) b.retention_days = Number(f.retentionDays) || 0
  if (Number(f.forceMaxAgeDays) !== (o.force_max_age_days ?? 0)) b.force_max_age_days = Number(f.forceMaxAgeDays) || 0
  return b
}

function ExtInput({ exts, onChange }: { exts: string[]; onChange(v: string[]): void }) {
  const { t } = useT()
  const inputId = useId()
  const [draft, setDraft] = useState('')
  const add = () => {
    const v = draft.trim().toLowerCase().replace(/^\./, '')
    if (v && !exts.includes(v)) onChange([...exts, v])
    setDraft('')
  }
  return (
    <div className={forms.field}>
      <label className={forms.label} htmlFor={inputId}>{t('adminA.allowedExts')}</label>
      <div className={styles.tags}>
        {exts.map((e) => (
          <span key={e} className={styles.tag}>
            {e}
            <button type="button" aria-label={t('adminA.removeExtAria', { ext: e })} onClick={() => onChange(exts.filter((x) => x !== e))}>×</button>
          </span>
        ))}
        <input
          id={inputId}
          className={styles.tagInput}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ',') {
              e.preventDefault()
              add()
            }
          }}
          onBlur={add}
          placeholder={t('adminA.extPlaceholder')}
        />
      </div>
    </div>
  )
}

export function GroupsPage() {
  const { t } = useT()
  const pushToast = useGlobal((s) => s.pushToast)
  const groupsQ = useAdminGroups()
  const policiesQ = useAdminPolicies()
  const create = useCreateGroup()
  const update = useUpdateGroup()
  const del = useDeleteGroup()
  const lifePreview = usePreviewGroupLifecycle()
  const lifeApply = useApplyGroupLifecycle()

  const groups = groupsQ.data?.items ?? []
  const policies = policiesQ.data?.items ?? []
  const [sel, setSel] = useState<number | 'new' | null>(null)
  const [form, setForm] = useState<FormState>(NEW_FORM)
  const [lifeMsg, setLifeMsg] = useState<string | null>(null)

  const current = typeof sel === 'number' ? groups.find((g) => g.id === sel) ?? null : null
  const builtin = !!current && (current.is_default || current.is_guest)
  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => setForm((f) => ({ ...f, [k]: v }))

  const selectGroup = (g: AdminGroup) => {
    setSel(g.id)
    setForm(formOf(g))
    setLifeMsg(null)
  }
  const selectNew = () => {
    setSel('new')
    setForm(NEW_FORM)
    setLifeMsg(null)
  }

  const submit = () => {
    if (sel === 'new') {
      create.mutate(createBody(form), { onSuccess: () => setSel(null) })
    } else if (current) {
      const body = patchBody(current, form)
      if (Object.keys(body).length === 0) return
      update.mutate({ id: current.id, body })
    }
  }

  const onLifecyclePreview = () => {
    if (!current) return
    setLifeMsg(null)
    lifePreview.mutate(current.id, {
      onSuccess: (p) => {
        if (p.cap_sec <= 0) setLifeMsg(t('adminA.lifecycleNoCap'))
        else {
          setLifeMsg(t('adminA.lifecyclePreviewResult', {
            perm: p.permanent_count,
            over: p.over_cap_count,
            total: p.total,
            cap: p.cap_sec,
          }))
        }
      },
    })
  }

  const onLifecycleApply = () => {
    if (!current) return
    if (!window.confirm(t('adminA.lifecycleApplyConfirm'))) return
    lifeApply.mutate(
      { id: current.id },
      {
        onSuccess: (r) => {
          const msg = t('adminA.lifecycleApplyResult', { updated: r.updated, skipped: r.skipped })
          setLifeMsg(msg)
          pushToast(msg)
        },
      },
    )
  }

  return (
    <div>
      <PageHeader
        kicker="USER GROUPS"
        title={t('adminA.groupsTitle')}
        extra={<Button variant="primary" onClick={selectNew}>{t('adminA.newGroup')}</Button>}
      />
      <AdminQueryGate query={groupsQ}>
        {() => (
          <div className={styles.split}>
            <div className={styles.list}>
              {groups.map((g) => (
                <button
                  key={g.id}
                  type="button"
                  className={[styles.row, sel === g.id && styles.rowActive].filter(Boolean).join(' ')}
                  onClick={() => selectGroup(g)}
                >
                  <span className={styles.rowName}>{g.name}</span>
                  {(g.is_default || g.is_guest) && (
                    <span className={styles.builtin}>{g.is_guest ? t('adminA.guest') : t('adminA.defaultGroup')}</span>
                  )}
                  {lifecycleBadges(g, t).map((b) => (
                    <span key={b} className={styles.lifeBadge}>{b}</span>
                  ))}
                  <span className={styles.rowCount}>{t('adminA.memberCount', { count: g.user_count })}</span>
                </button>
              ))}
            </div>
            <div className={styles.detail}>
              {sel === null ? (
                <EmptyState title={t('adminA.selectOrCreate')} desc={t('adminA.selectOrCreateDesc')} />
              ) : (
                <div className={styles.form}>
                  <Input
                    label={t('adminA.groupName')}
                    value={form.name}
                    disabled={builtin}
                    extra={builtin ? <span className={forms.hint}>{t('adminA.builtinNameLocked')}</span> : undefined}
                    onChange={(e) => set('name', e.target.value)}
                  />
                  <div className={styles.grid2}>
                    <Input
                      label={t('adminA.quotaGB')}
                      type="number"
                      min={0}
                      value={form.quotaGB}
                      onChange={(e) => set('quotaGB', e.target.value)}
                    />
                    <Input
                      label={t('adminA.maxFileMB')}
                      type="number"
                      min={0}
                      value={form.maxMB}
                      onChange={(e) => set('maxMB', e.target.value)}
                    />
                  </div>
                  <Input
                    label={t('adminA.bandwidthQuotaGB')}
                    type="number"
                    min={0}
                    value={form.bwGB}
                    extra={<span className={forms.hint}>{t('adminA.bandwidthQuotaHint')}</span>}
                    onChange={(e) => set('bwGB', e.target.value)}
                  />
                  <div className={styles.grid3}>
                    <Input
                      label={t('adminA.ratePerMin')}
                      type="number"
                      min={0}
                      value={form.perMin}
                      onChange={(e) => set('perMin', e.target.value)}
                    />
                    <Input
                      label={t('adminA.ratePerHour')}
                      type="number"
                      min={0}
                      value={form.perHour}
                      onChange={(e) => set('perHour', e.target.value)}
                    />
                    <Input
                      label={t('adminA.ratePerDay')}
                      type="number"
                      min={0}
                      value={form.perDay}
                      onChange={(e) => set('perDay', e.target.value)}
                    />
                  </div>
                  <ExtInput exts={form.exts} onChange={(v) => set('exts', v)} />
                  <div className={forms.field}>
                    <span className={forms.label}>{t('adminA.lifecycleSection')}</span>
                    <p className={forms.hint}>{t('adminA.lifecycleHint')}</p>
                    <p className={forms.hint}>{t('adminA.lifecycleSecHint')}</p>
                    <p className={forms.hint}>{t('adminA.lifecycleStockNote')}</p>
                  </div>
                  <div className={styles.grid2}>
                    <Input
                      label={t('adminA.defaultExpiresSec')}
                      type="number"
                      min={0}
                      value={form.defaultExpires}
                      onChange={(e) => set('defaultExpires', e.target.value)}
                    />
                    <Input
                      label={t('adminA.maxExpiresSec')}
                      type="number"
                      min={0}
                      value={form.maxExpires}
                      onChange={(e) => set('maxExpires', e.target.value)}
                    />
                  </div>
                  <div className={styles.grid2}>
                    <Input
                      label={t('adminA.defaultMaxViews')}
                      type="number"
                      min={0}
                      value={form.defaultMaxViews}
                      onChange={(e) => set('defaultMaxViews', e.target.value)}
                    />
                    <Input
                      label={t('adminA.maxMaxViews')}
                      type="number"
                      min={0}
                      value={form.maxMaxViews}
                      onChange={(e) => set('maxMaxViews', e.target.value)}
                    />
                  </div>
                  <div className={styles.grid2}>
                    <Input
                      label={t('adminA.retentionDays')}
                      type="number"
                      min={0}
                      value={form.retentionDays}
                      onChange={(e) => set('retentionDays', e.target.value)}
                    />
                    <Input
                      label={t('adminA.forceMaxAgeDays')}
                      type="number"
                      min={0}
                      value={form.forceMaxAgeDays}
                      onChange={(e) => set('forceMaxAgeDays', e.target.value)}
                    />
                  </div>
                  <div className={forms.field}>
                    <span className={forms.label}>{t('adminA.allowedPolicies')}</span>
                    <div className={styles.policies}>
                      {policiesQ.isError ? (
                        <span className={forms.hint}>{t('adminA.policiesLoadFailed')}</span>
                      ) : policies.length === 0 ? (
                        <span className={forms.hint}>{t('adminA.noPolicies')}</span>
                      ) : null}
                      {policies.map((p) => (
                        <label key={p.id} className={styles.check}>
                          <input
                            type="checkbox"
                            checked={form.policyIds.includes(p.id)}
                            onChange={(e) =>
                              set(
                                'policyIds',
                                e.target.checked
                                  ? [...form.policyIds, p.id]
                                  : form.policyIds.filter((x) => x !== p.id),
                              )
                            }
                          />
                          {p.name}
                        </label>
                      ))}
                    </div>
                  </div>
                  {current && (
                    <div className={styles.lifeActions}>
                      <Button
                        variant="secondary"
                        disabled={lifePreview.isPending || lifeApply.isPending}
                        onClick={onLifecyclePreview}
                      >
                        {t('adminA.lifecyclePreview')}
                      </Button>
                      <Button
                        variant="secondary"
                        disabled={lifePreview.isPending || lifeApply.isPending}
                        onClick={onLifecycleApply}
                      >
                        {t('adminA.lifecycleApply')}
                      </Button>
                      {lifeMsg && <p className={forms.hint}>{lifeMsg}</p>}
                    </div>
                  )}
                  <div className={styles.actions}>
                    <Button
                      variant="primary"
                      disabled={create.isPending || update.isPending || del.isPending}
                      onClick={submit}
                    >
                      {t('common.save')}
                    </Button>
                    {current && !builtin && (
                      <InlineConfirm
                        label={t('common.delete')}
                        onConfirm={() => del.mutate(current.id, { onSuccess: () => setSel(null) })}
                      />
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </AdminQueryGate>
    </div>
  )
}
