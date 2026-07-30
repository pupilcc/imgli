import { useState } from 'react'
import { useAdminImages, useAdminPolicies, useDeleteAdminImage, useSetImageWhitelist } from '../../../api/adminHooks'
import type { AdminImageItem } from '../../../api/types'
import { useT } from '../../../i18n'
import { formatBytes } from '../../../lib/format'
import { useAdminSearchParam } from '../../../lib/useAdminSearchParam'
import { PageHeader } from '../../../shell/PageHeader'
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
  const { params, setParam } = useAdminSearchParam()
  const user = Number(params.get('user')) || undefined
  const status = params.get('status') ?? ''
  const policy = Number(params.get('policy')) || undefined
  const page = Number(params.get('page')) || 1

  const images = useAdminImages({ user, status: status || undefined, policy, page })
  const policiesQ = useAdminPolicies()
  const wl = useSetImageWhitelist()
  const delM = useDeleteAdminImage()
  const [detail, setDetail] = useState<AdminImageItem | null>(null)

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
                        {it.status === 'pending' && <span className={styles.bWarn}>{t('adminA.statusPending')}</span>}
                        {it.status === 'rejected' && <span className={styles.bErr}>{t('adminA.statusRejected')}</span>}
                        {it.is_whitelisted && <span className={styles.bWl}>WL</span>}
                      </div>
                      <div className={styles.hoverBar}>
                        <ArmedButton
                          title={it.is_whitelisted ? t('adminA.unwhitelist') : t('adminA.whitelist')}
                          armedTitle={it.is_whitelisted ? t('adminA.confirmUnwhitelist') : t('adminA.confirmWhitelist')}
                          className={styles.quickBtn}
                          onConfirm={() => wl.mutate({ key: it.key, on: !it.is_whitelisted })}
                        >
                          {t('adminA.whitelistGlyph')}
                        </ArmedButton>
                        <ArmedButton
                          title={t('common.delete')}
                          armedTitle={t('adminA.confirmDelete')}
                          className={[styles.quickBtn, styles.quickDanger].join(' ')}
                          armedClassName={styles.quickArmed}
                          onConfirm={() => delM.mutate(it.key)}
                        >
                          ×
                        </ArmedButton>
                      </div>
                    </div>
                    <div className={styles.meta}>
                      <span className={styles.metaName}>{it.name}</span>
                      <span className={styles.metaUser}>{it.username}</span>
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
