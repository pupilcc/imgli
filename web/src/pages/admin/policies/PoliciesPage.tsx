import { useState } from 'react'
import { useAdminPolicies, useCreatePolicy, useDeletePolicy, useTestPolicy, useUpdatePolicy } from '../../../api/adminHooks'
import type { AdminPolicy } from '../../../api/types'
import { useT } from '../../../i18n'
import { PageHeader } from '../../../shell/PageHeader'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { InlineConfirm } from '../../../ui/InlineConfirm'
import { Input } from '../../../ui/Input'
import { Segmented } from '../../../ui/Segmented'
import { Toggle } from '../../../ui/Toggle'
import { AdminQueryGate } from '../ui/AdminQueryGate'
import forms from '../ui/adminForms.module.css'
import styles from './PoliciesPage.module.css'

interface FormState {
  name: string
  driver: 'local' | 's3' | 'webdav'
  root: string
  s3Endpoint: string
  s3Region: string
  s3Bucket: string
  s3AKID: string
  s3Secret: string
  s3PathStyle: 'true' | 'false'
  s3Prefix: string
  s3PresignDomain: string
  davEndpoint: string
  davUsername: string
  davPassword: string
  cdn: string
  tpl: string
  enabled: boolean
}

const NEW_FORM: FormState = {
  name: '',
  driver: 'local',
  root: '',
  s3Endpoint: '',
  s3Region: '',
  s3Bucket: '',
  s3AKID: '',
  s3Secret: '',
  s3PathStyle: 'false',
  s3Prefix: '',
  s3PresignDomain: '',
  davEndpoint: '',
  davUsername: '',
  davPassword: '',
  cdn: '',
  tpl: '{Y}/{m}/{d}/{uniqid}.{ext}',
  enabled: true,
}

function formOf(p: AdminPolicy): FormState {
  let root = ''
  let s3Endpoint = ''
  let s3Region = ''
  let s3Bucket = ''
  let s3AKID = ''
  let s3Secret = ''
  let s3PathStyle: 'true' | 'false' = 'false'
  let s3Prefix = ''
  let s3PresignDomain = ''
  let davEndpoint = ''
  let davUsername = ''
  let davPassword = ''
  try {
    const cfg = JSON.parse(p.config) as {
      root?: string
      endpoint?: string
      region?: string
      bucket?: string
      access_key_id?: string
      secret_access_key?: string
      path_style?: string
      prefix?: string
      presign_domain?: string
      username?: string
      password?: string
    }
    if (p.driver === 's3') {
      s3Endpoint = cfg.endpoint ?? ''
      s3Region = cfg.region ?? ''
      s3Bucket = cfg.bucket ?? ''
      s3AKID = cfg.access_key_id ?? ''
      s3Secret = cfg.secret_access_key ?? ''
      s3PathStyle = cfg.path_style === 'true' ? 'true' : 'false'
      s3Prefix = cfg.prefix ?? ''
      s3PresignDomain = cfg.presign_domain ?? ''
    } else if (p.driver === 'webdav') {
      davEndpoint = cfg.endpoint ?? ''
      davUsername = cfg.username ?? ''
      davPassword = cfg.password ?? ''
    } else {
      root = cfg.root ?? ''
    }
  } catch {
    /* ignore bad config */
  }
  return {
    name: p.name,
    driver: p.driver === 's3' ? 's3' : p.driver === 'webdav' ? 'webdav' : 'local',
    root,
    s3Endpoint,
    s3Region,
    s3Bucket,
    s3AKID,
    s3Secret,
    s3PathStyle,
    s3Prefix,
    s3PresignDomain,
    davEndpoint,
    davUsername,
    davPassword,
    cdn: p.cdn_domain,
    tpl: p.path_template,
    enabled: p.enabled,
  }
}

export function PoliciesPage() {
  const { t } = useT()
  const policiesQ = useAdminPolicies()
  const create = useCreatePolicy()
  const update = useUpdatePolicy()
  const del = useDeletePolicy()
  const test = useTestPolicy()

  const policies = policiesQ.data?.items ?? []
  const [sel, setSel] = useState<number | 'new' | null>(null)
  const [form, setForm] = useState<FormState>(NEW_FORM)
  const [testMsg, setTestMsg] = useState<string | null>(null)

  const current = typeof sel === 'number' ? policies.find((p) => p.id === sel) ?? null : null
  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => setForm((f) => ({ ...f, [k]: v }))

  const selectPolicy = (p: AdminPolicy) => {
    setSel(p.id)
    setForm(formOf(p))
    setTestMsg(null)
  }
  const selectNew = () => {
    setSel('new')
    setForm(NEW_FORM)
    setTestMsg(null)
  }

  const config =
    form.driver === 's3'
      ? JSON.stringify({
          endpoint: form.s3Endpoint.trim(),
          region: form.s3Region.trim(),
          bucket: form.s3Bucket.trim(),
          access_key_id: form.s3AKID.trim(),
          secret_access_key: form.s3Secret,
          path_style: form.s3PathStyle,
          prefix: form.s3Prefix.trim(),
          presign_domain: form.s3PresignDomain.trim(),
        })
      : form.driver === 'webdav'
        ? JSON.stringify({
            endpoint: form.davEndpoint.trim(),
            username: form.davUsername.trim(),
            password: form.davPassword,
          })
        : JSON.stringify({ root: form.root.trim() })

  const submit = () => {
    setTestMsg(null)
    if (sel === 'new') {
      create.mutate(
        {
          name: form.name.trim(),
          driver: form.driver,
          config,
          cdn_domain: form.cdn,
          path_template: form.tpl,
          enabled: form.enabled,
        },
        { onSuccess: () => setSel(null) },
      )
    } else if (current) {
      update.mutate({
        id: current.id,
        body: { name: form.name.trim(), config, cdn_domain: form.cdn, path_template: form.tpl, enabled: form.enabled },
      })
    }
  }

  const runTest = () => {
    if (!current) return
    setTestMsg(null)
    test.mutate(current.id, { onSuccess: (r) => setTestMsg(t('adminB.connectedMs', { ms: r.latency_ms })) })
  }

  const driverLabel =
    form.driver === 's3' ? 'S3' : form.driver === 'webdav' ? 'WebDAV' : t('adminB.driverLocal')

  return (
    <div>
      <PageHeader
        kicker="STORAGE POLICY"
        title={t('adminB.policiesTitle')}
        extra={<Button variant="primary" onClick={selectNew}>{t('adminB.newPolicy')}</Button>}
      />
      <AdminQueryGate query={policiesQ}>
        {() => (
        <div className={styles.split}>
          <div className={styles.list}>
            {policies.map((p) => (
              <button
                key={p.id}
                type="button"
                className={[styles.row, sel === p.id && styles.rowActive].filter(Boolean).join(' ')}
                onClick={() => selectPolicy(p)}
              >
                <span className={styles.rowName}>{p.name}</span>
                {!p.enabled && <span className={styles.off}>{t('adminB.disabled')}</span>}
                <span className={styles.rowCount}>{t('adminB.fileCount', { count: p.file_count })}</span>
              </button>
            ))}
          </div>
          <div className={styles.detail}>
            {sel === null ? (
              <EmptyState title={t('adminB.selectOrCreatePolicy')} desc={t('adminB.selectOrCreatePolicyDesc')} />
            ) : (
              <div className={styles.form}>
                <Input label={t('adminB.name')} value={form.name} onChange={(e) => set('name', e.target.value)} />
                <div className={forms.field}>
                  <span className={forms.label}>{t('adminB.driver')}</span>
                  {sel === 'new' ? (
                    <Segmented<'local' | 's3' | 'webdav'>
                      options={[
                        { value: 'local', label: t('adminB.driverLocal') },
                        { value: 's3', label: 'S3' },
                        { value: 'webdav', label: 'WebDAV' },
                      ]}
                      value={form.driver}
                      onChange={(v) => set('driver', v)}
                    />
                  ) : (
                    <div className={styles.driver}>{driverLabel}</div>
                  )}
                </div>
                {form.driver === 'local' ? (
                  <Input label={t('adminB.storagePath')} value={form.root} placeholder="/data/uploads" onChange={(e) => set('root', e.target.value)} />
                ) : form.driver === 'webdav' ? (
                  <>
                    <Input
                      label="Endpoint"
                      value={form.davEndpoint}
                      placeholder="https://dav.example.com/imgli"
                      onChange={(e) => set('davEndpoint', e.target.value)}
                    />
                    <Input
                      label={t('adminB.username')}
                      value={form.davUsername}
                      placeholder={t('adminB.davUserPlaceholder')}
                      onChange={(e) => set('davUsername', e.target.value)}
                    />
                    <Input
                      label={t('adminB.password')}
                      value={form.davPassword}
                      extra={<span className={forms.hint}>{t('adminB.secretMaskHint')}</span>}
                      onChange={(e) => set('davPassword', e.target.value)}
                      onFocus={(e) => e.target.select()}
                    />
                  </>
                ) : (
                  <>
                    <Input
                      label="Endpoint"
                      value={form.s3Endpoint}
                      placeholder="s3.us-east-1.amazonaws.com"
                      onChange={(e) => set('s3Endpoint', e.target.value)}
                    />
                    <Input
                      label="Region"
                      value={form.s3Region}
                      placeholder="us-east-1"
                      onChange={(e) => set('s3Region', e.target.value)}
                    />
                    <Input label="Bucket" value={form.s3Bucket} onChange={(e) => set('s3Bucket', e.target.value)} />
                    <Input label="AccessKey ID" value={form.s3AKID} onChange={(e) => set('s3AKID', e.target.value)} />
                    <Input
                      label="AccessKey Secret"
                      value={form.s3Secret}
                      extra={<span className={forms.hint}>{t('adminB.secretMaskHint')}</span>}
                      onChange={(e) => set('s3Secret', e.target.value)}
                      onFocus={(e) => e.target.select()}
                    />
                    <div className={forms.field}>
                      <span className={forms.label}>{t('adminB.pathStyle')}</span>
                      <Segmented<'true' | 'false'>
                        options={[
                          { value: 'false', label: t('adminB.pathStyleVirtual') },
                          { value: 'true', label: t('adminB.pathStylePath') },
                        ]}
                        value={form.s3PathStyle}
                        onChange={(v) => set('s3PathStyle', v)}
                      />
                    </div>
                    <Input
                      label={t('adminB.prefix')}
                      value={form.s3Prefix}
                      placeholder="imgli/"
                      onChange={(e) => set('s3Prefix', e.target.value)}
                    />
                    <Input
                      label={t('adminB.presignDomain')}
                      value={form.s3PresignDomain}
                      placeholder="https://s3.img.li"
                      extra={<span className={forms.hint}>{t('adminB.presignDomainHint')}</span>}
                      onChange={(e) => set('s3PresignDomain', e.target.value)}
                    />
                  </>
                )}
                <Input
                  label={t('adminB.cdnDomain')}
                  value={form.cdn}
                  placeholder={t('adminB.cdnDomainPlaceholder')}
                  onChange={(e) => set('cdn', e.target.value)}
                />
                <Input label={t('adminB.pathTemplate')} value={form.tpl} onChange={(e) => set('tpl', e.target.value)} />
                <div className={styles.toggleRow}>
                  <span className={forms.label}>{t('adminB.enabled')}</span>
                  <Toggle checked={form.enabled} onChange={(v) => set('enabled', v)} />
                </div>
                <div className={styles.actions}>
                  <Button variant="primary" disabled={create.isPending || update.isPending || del.isPending} onClick={submit}>{t('common.save')}</Button>
                  {current && (
                    <>
                      <Button variant="secondary" disabled={test.isPending} onClick={runTest}>{t('adminB.testConnection')}</Button>
                      <InlineConfirm label={t('common.delete')} onConfirm={() => del.mutate(current.id, { onSuccess: () => setSel(null) })} />
                    </>
                  )}
                  {testMsg && <span className={styles.testOk}>{testMsg}</span>}
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
