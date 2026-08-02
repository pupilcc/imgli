import { useState } from 'react'
import {
  useAdminImages,
  useAdminPolicies,
  useDeleteAdminImage,
  usePurgeAdminImage,
  useSetImageWhitelist,
} from '../../../api/adminHooks'
import type { AdminImageItem } from '../../../api/types'
import { useT } from '../../../i18n'
import { formatBytes } from '../../../lib/format'
import { useAdminSearchParam } from '../../../lib/useAdminSearchParam'
import { PageHeader } from '../../../shell/PageHeader'
import { useGlobal } from '../../../store'
import { ArmedButton } from '../../../ui/ArmedButton'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { AdminQueryGate } from '../ui/AdminQueryGate'
import { Pager } from '../ui/Pager'
import { AdminImageDetail } from './AdminImageDetail'
import forms from '../ui/adminForms.module.css'
import styles from './ImagesAdminPage.module.css'

export function ImagesAdminPage() {
  const { t } = useT()
  const pushToast = useGlobal((s) => s.pushToast)
  const { params, setParam } = useAdminSearchParam()
  const user = Number(params.get('user')) || undefined
  const status = params.get('status') ?? ''
  const policy = Number(params.get('policy')) || undefined
  const deleted = params.get('deleted') || 'live'
  const page = Number(params.get('page')) || 1

  const images = useAdminImages({
    user,
    status: status || undefined,
    policy,
    deleted: deleted === 'live' ? undefined : deleted,
    page,
  })
  const policiesQ = useAdminPolicies()
  const wl = useSetImageWhitelist()
  const delM = useDeleteAdminImage()
  const purgeM = usePurgeAdminImage()
  const [detail, setDetail] = useState<AdminImageItem | null>(null)

  const onSoftOrGuestDelete = (it: AdminImageItem) => {
    // 游客：服务端会自动 permanent；已在回收站：走 purge
    if (it.in_trash || it.user_id == null) {
      purgeM.mutate(it.key, {
        onSuccess: (res) => {
          if (res.object_retained) pushToast(t('adminA.toastPurgedRetained'))
          else if (res.physical_queued) pushToast(t('adminA.toastPurgedQueued'))
          else pushToast(t('adminA.toastPurgedNoQueue'))
        },
      })
      return
    }
    delM.mutate(it.key, {
      onSuccess: (res) => {
        if (res.permanent) {
          if (res.object_retained) pushToast(t('adminA.toastPurgedRetained'))
          else if (res.physical_queued) pushToast(t('adminA.toastPurgedQueued'))
          else pushToast(t('adminA.toastPurged'))
        } else pushToast(t('adminA.toastMovedToTrash'))
      },
    })
  }

  return (
    <div>
      <PageHeader
        kicker="ALL IMAGES"
        title={t('adminA.imagesTitle')}
        extra={
          <div className={forms.filters}>
            {user && (
              <span className={styles.chip}>
                {t('adminA.userFilterChip', { user })}
                <button type="button" aria-label={t('adminA.clearUserFilterAria')} onClick={() => setParam('user', '')}>
                  ×
                </button>
              </span>
            )}
            <select
              value={deleted}
              onChange={(e) => setParam('deleted', e.target.value === 'live' ? '' : e.target.value)}
              className={forms.select}
              aria-label={t('adminA.filterScopeAria')}
            >
              <option value="live">{t('adminA.scopeLive')}</option>
              <option value="trash">{t('adminA.scopeTrash')}</option>
              <option value="all">{t('adminA.scopeAll')}</option>
            </select>
            <select value={status} onChange={(e) => setParam('status', e.target.value)} className={forms.select} aria-label={t('adminA.filterStatusAria')}>
              <option value="">{t('adminA.allStatuses')}</option>
              <option value="normal">{t('adminA.statusNormal')}</option>
              <option value="pending">{t('adminA.statusPending')}</option>
              <option value="rejected">{t('adminA.statusRejected')}</option>
            </select>
            <select value={policy ?? ''} onChange={(e) => setParam('policy', e.target.value)} className={forms.select} aria-label={t('adminA.filterPolicyAria')}>
              <option value="">{t('adminA.allPolicies')}</option>
              {(policiesQ.data?.items ?? []).map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </div>
        }
      />
      <AdminQueryGate query={images}>
        {(data) =>
          data.items.length === 0 ? (
            data.total > 0 ? (
              <EmptyState badge="✓" title={t('adminA.pageCleared')} desc={t('adminA.pageClearedImagesDesc')}>
                <Button variant="secondary" onClick={() => setParam('page', '')}>
                  {t('adminA.backToPage1')}
                </Button>
              </EmptyState>
            ) : (
              <EmptyState title={t('adminA.noMatchingImages')} desc={t('adminA.noMatchingImagesDesc')} />
            )
          ) : (
            <>
              <div className={styles.grid}>
                {data.items.map((it) => (
                  <div key={it.key} className={styles.card} onClick={() => setDetail(it)}>
                    <div className={styles.thumbBox}>
                      <img className={styles.thumb} src={it.links.thumbnail_url} alt={it.name} loading="lazy" />
                      <div className={styles.badges}>
                        {it.in_trash && <span className={styles.bErr}>{t('adminA.trashBadge')}</span>}
                        {it.status === 'pending' && <span className={styles.bWarn}>{t('adminA.statusPending')}</span>}
                        {it.status === 'rejected' && <span className={styles.bErr}>{t('adminA.statusRejected')}</span>}
                        {it.is_whitelisted && <span className={styles.bWl}>WL</span>}
                      </div>
                      <div className={styles.hoverBar}>
                        {!it.in_trash && (
                          <ArmedButton
                            title={it.is_whitelisted ? t('adminA.unwhitelist') : t('adminA.whitelist')}
                            armedTitle={it.is_whitelisted ? t('adminA.confirmUnwhitelist') : t('adminA.confirmWhitelist')}
                            className={styles.quickBtn}
                            armedClassName={styles.quickArmed}
                            armedChildren={t('adminA.confirmShort')}
                            onConfirm={() => wl.mutate({ key: it.key, on: !it.is_whitelisted })}
                          >
                            {t('adminA.whitelistGlyph')}
                          </ArmedButton>
                        )}
                        <ArmedButton
                          title={
                            it.in_trash || it.user_id == null
                              ? t('adminA.purgePermanent')
                              : t('adminA.moveToTrash')
                          }
                          armedTitle={
                            it.in_trash || it.user_id == null
                              ? t('adminA.confirmPurge')
                              : t('adminA.confirmMoveToTrash')
                          }
                          className={[styles.quickBtn, styles.quickDanger].join(' ')}
                          armedClassName={styles.quickArmed}
                          armedChildren={t('adminA.confirmShort')}
                          onConfirm={() => onSoftOrGuestDelete(it)}
                        >
                          ×
                        </ArmedButton>
                      </div>
                    </div>
                    <div className={styles.meta}>
                      <span className={styles.metaName}>{it.name}</span>
                      <span className={styles.metaUser}>
                        {it.user_id == null ? t('adminA.guestUploader') : it.username}
                      </span>
                      <span className={styles.metaPolicy} title={it.path || undefined}>
                        {it.policy_name || it.policy_driver || '—'}
                      </span>
                      <span className={styles.metaSize}>{formatBytes(it.size)}</span>
                    </div>
                  </div>
                ))}
              </div>
              <p className={styles.stat}>{t('adminA.imagesTotal', { total: data.total })}</p>
              <Pager
                page={page}
                limit={data.limit}
                total={data.total}
                onPage={(p) => setParam('page', p > 1 ? String(p) : '')}
              />
            </>
          )
        }
      </AdminQueryGate>
      <AdminImageDetail item={detail} onClose={() => setDetail(null)} />
    </div>
  )
}
