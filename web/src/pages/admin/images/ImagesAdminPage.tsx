import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router'
import { useAdminImages, useAdminPolicies, useDeleteAdminImage, useSetImageWhitelist } from '../../../api/adminHooks'
import type { AdminImageItem } from '../../../api/types'
import { useT } from '../../../i18n'
import { formatBytes } from '../../../lib/format'
import { PageHeader } from '../../../shell/PageHeader'
import { Button } from '../../../ui/Button'
import { EmptyState } from '../../../ui/EmptyState'
import { Skeleton } from '../../../ui/Skeleton'
import { AdminError } from '../ui/AdminError'
import { Pager } from '../ui/Pager'
import { AdminImageDetail } from './AdminImageDetail'
import styles from './ImagesAdminPage.module.css'

/** 两击确认小按钮(沿前台 QuickDel 模式)。 */
function ArmedBtn({ title, armedTitle, onConfirm, danger, glyph }: { title: string; armedTitle: string; onConfirm(): void; danger?: boolean; glyph: string }) {
  const [armed, setArmed] = useState(false)
  useEffect(() => {
    if (!armed) return
    const t = setTimeout(() => setArmed(false), 2500)
    return () => clearTimeout(t)
  }, [armed])
  return (
    <button
      type="button"
      title={armed ? armedTitle : title}
      className={[styles.quickBtn, danger && styles.quickDanger, armed && danger && styles.quickArmed].filter(Boolean).join(' ')}
      onClick={(e) => {
        e.stopPropagation()
        if (armed) {
          setArmed(false)
          onConfirm()
        } else setArmed(true)
      }}
    >
      {glyph}
    </button>
  )
}

export function ImagesAdminPage() {
  const { t } = useT()
  const [params, setParams] = useSearchParams()
  const user = Number(params.get('user')) || undefined
  const status = params.get('status') ?? ''
  const policy = Number(params.get('policy')) || undefined
  const page = Number(params.get('page')) || 1

  const setParam = (key: string, value: string) => {
    setParams((p) => {
      const n = new URLSearchParams(p)
      if (value) n.set(key, value)
      else n.delete(key)
      if (key !== 'page') n.delete('page')
      return n
    })
  }

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
          <div className={styles.filters}>
            {user && (
              <span className={styles.chip}>
                {t('adminA.userFilterChip', { user })}
                <button type="button" aria-label={t('adminA.clearUserFilterAria')} onClick={() => setParam('user', '')}>
                  ×
                </button>
              </span>
            )}
            <select value={status} onChange={(e) => setParam('status', e.target.value)} className={styles.select} aria-label={t('adminA.filterStatusAria')}>
              <option value="">{t('adminA.allStatuses')}</option>
              <option value="normal">{t('adminA.statusNormal')}</option>
              <option value="pending">{t('adminA.statusPending')}</option>
              <option value="rejected">{t('adminA.statusRejected')}</option>
            </select>
            <select value={policy ?? ''} onChange={(e) => setParam('policy', e.target.value)} className={styles.select} aria-label={t('adminA.filterPolicyAria')}>
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
      {images.isError ? (
        <AdminError onRetry={() => images.refetch()} />
      ) : !images.data ? (
        <Skeleton height={220} />
      ) : images.data.items.length === 0 ? (
        images.data.total > 0 ? (
          <EmptyState badge="✓" title={t('adminA.pageCleared')} desc={t('adminA.pageClearedImagesDesc')}>
            <Button variant="secondary" onClick={() => setParam('page', '')}>{t('adminA.backToPage1')}</Button>
          </EmptyState>
        ) : (
          <EmptyState title={t('adminA.noMatchingImages')} desc={t('adminA.noMatchingImagesDesc')} />
        )
      ) : (
        <>
          <div className={styles.grid}>
            {images.data.items.map((it) => (
              <div key={it.key} className={styles.card} onClick={() => setDetail(it)}>
                <div className={styles.thumbBox}>
                  <img className={styles.thumb} src={it.links.thumbnail_url} alt={it.name} loading="lazy" />
                  <div className={styles.badges}>
                    {it.status === 'pending' && <span className={styles.bWarn}>{t('adminA.statusPending')}</span>}
                    {it.status === 'rejected' && <span className={styles.bErr}>{t('adminA.statusRejected')}</span>}
                    {it.is_whitelisted && <span className={styles.bWl}>WL</span>}
                  </div>
                  <div className={styles.hoverBar}>
                    <ArmedBtn
                      title={it.is_whitelisted ? t('adminA.unwhitelist') : t('adminA.whitelist')}
                      armedTitle={it.is_whitelisted ? t('adminA.confirmUnwhitelist') : t('adminA.confirmWhitelist')}
                      glyph={t('adminA.whitelistGlyph')}
                      onConfirm={() => wl.mutate({ key: it.key, on: !it.is_whitelisted })}
                    />
                    <ArmedBtn
                      title={t('common.delete')}
                      armedTitle={t('adminA.confirmDelete')}
                      glyph="×"
                      danger
                      onConfirm={() => delM.mutate(it.key)}
                    />
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
          <p className={styles.stat}>{t('adminA.imagesTotal', { total: images.data.total })}</p>
          <Pager page={page} limit={images.data.limit} total={images.data.total} onPage={(p) => setParam('page', p > 1 ? String(p) : '')} />
        </>
      )}
      <AdminImageDetail item={detail} onClose={() => setDetail(null)} />
    </div>
  )
}
