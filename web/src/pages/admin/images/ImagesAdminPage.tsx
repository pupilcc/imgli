import { useEffect, useState } from 'react'
import {
  useAdminImages,
  useAdminImagesBatch,
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

function purgeToast(
  t: (k: string, v?: Record<string, string | number>) => string,
  res: { physical_queued?: boolean; object_retained?: boolean },
) {
  if (res.object_retained) return t('adminA.toastPurgedRetained')
  if (res.physical_queued) return t('adminA.toastPurgedQueued')
  return t('adminA.toastPurgedNoQueue')
}

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
  const batchM = useAdminImagesBatch()
  const [detail, setDetail] = useState<AdminImageItem | null>(null)
  const [selected, setSelected] = useState<Set<string>>(() => new Set())
  const [trashArmed, setTrashArmed] = useState(false)
  const [purgeArmed, setPurgeArmed] = useState(false)

  // 筛选/翻页时清空选择，避免跨页误删
  useEffect(() => {
    setSelected(new Set())
    setTrashArmed(false)
    setPurgeArmed(false)
  }, [user, status, policy, deleted, page])

  useEffect(() => {
    if (!trashArmed) return
    const timer = setTimeout(() => setTrashArmed(false), 2500)
    return () => clearTimeout(timer)
  }, [trashArmed])
  useEffect(() => {
    if (!purgeArmed) return
    const timer = setTimeout(() => setPurgeArmed(false), 2500)
    return () => clearTimeout(timer)
  }, [purgeArmed])

  const onSoftDelete = (it: AdminImageItem) => {
    if (it.in_trash || it.user_id == null) {
      purgeM.mutate(it.key, {
        onSuccess: (res) => pushToast(purgeToast(t, res)),
      })
      return
    }
    delM.mutate(it.key, {
      onSuccess: (res) => {
        if (res.permanent) pushToast(purgeToast(t, res))
        else pushToast(t('adminA.toastMovedToTrash'))
      },
    })
  }

  const onPurge = (it: AdminImageItem) => {
    purgeM.mutate(it.key, {
      onSuccess: (res) => pushToast(purgeToast(t, res)),
    })
  }

  const selectPage = (items: AdminImageItem[]) => {
    setSelected(new Set(items.map((i) => i.key)))
  }

  const runBatch = (action: 'trash' | 'purge') => {
    const keys = [...selected]
    if (keys.length === 0) return
    setTrashArmed(false)
    setPurgeArmed(false)
    const CHUNK = 100
    ;(async () => {
      let ok = 0
      let failed = 0
      try {
        for (let i = 0; i < keys.length; i += CHUNK) {
          const chunk = keys.slice(i, i + CHUNK)
          const data = await batchM.mutateAsync({ keys: chunk, action })
          for (const r of data.results) {
            if (r.ok) ok++
            else failed++
          }
        }
        pushToast(
          failed === 0
            ? t('adminA.batchImagesDone', { ok, action: action === 'purge' ? t('adminA.verbPurge') : t('adminA.verbTrash') })
            : t('adminA.batchImagesPartial', { ok, failed }),
        )
        setSelected(new Set())
      } catch {
        pushToast(t('adminA.batchImagesFailed'))
      }
    })()
  }

  const busy = delM.isPending || purgeM.isPending || batchM.isPending || wl.isPending
  const items = images.data?.items ?? []
  const canSoftBatch = selected.size > 0 && [...selected].some((k) => {
    const it = items.find((i) => i.key === k)
    return it && !it.in_trash && it.user_id != null
  })

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
              <div className={styles.toolbar}>
                <button type="button" className={styles.toolBtn} onClick={() => selectPage(data.items)}>
                  {t('adminA.selectPage')}
                </button>
                {selected.size > 0 && (
                  <button type="button" className={styles.toolBtn} onClick={() => setSelected(new Set())}>
                    {t('adminA.clearSelection')}
                  </button>
                )}
                <span className={styles.toolHint}>{t('adminA.selectHint')}</span>
              </div>
              <div className={styles.grid}>
                {data.items.map((it) => {
                  const showSoft = !it.in_trash && it.user_id != null
                  const isSel = selected.has(it.key)
                  return (
                    <div
                      key={it.key}
                      className={[styles.card, isSel && styles.cardSelected].filter(Boolean).join(' ')}
                      onClick={() => setDetail(it)}
                    >
                      <div className={styles.thumbBox}>
                        <img className={styles.thumb} src={it.links.thumbnail_url} alt={it.name} loading="lazy" />
                        <label
                          className={styles.check}
                          onClick={(e) => e.stopPropagation()}
                          title={t('adminA.selectImage')}
                        >
                          <input
                            type="checkbox"
                            checked={isSel}
                            onChange={() => {
                              setSelected((prev) => {
                                const next = new Set(prev)
                                if (next.has(it.key)) next.delete(it.key)
                                else next.add(it.key)
                                return next
                              })
                            }}
                          />
                        </label>
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
                          {showSoft && (
                            <ArmedButton
                              title={t('adminA.moveToTrash')}
                              armedTitle={t('adminA.confirmMoveToTrash')}
                              className={[styles.quickBtn, styles.quickDanger].join(' ')}
                              armedClassName={styles.quickArmed}
                              armedChildren={t('adminA.confirmShort')}
                              onConfirm={() => onSoftDelete(it)}
                            >
                              ×
                            </ArmedButton>
                          )}
                          <ArmedButton
                            title={t('adminA.purgePermanent')}
                            armedTitle={t('adminA.confirmPurge')}
                            className={[styles.quickBtn, styles.quickPurge].join(' ')}
                            armedClassName={styles.quickArmed}
                            armedChildren={t('adminA.confirmShort')}
                            onConfirm={() => onPurge(it)}
                          >
                            ⌫
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
                  )
                })}
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

      {selected.size > 0 && (
        <div className={styles.batchBar} role="toolbar" aria-label={t('adminA.batchToolbar')}>
          <span className={styles.batchCount}>{t('adminA.selectedCount', { count: selected.size })}</span>
          {canSoftBatch && (
            <button
              type="button"
              className={[styles.batchAct, trashArmed && styles.batchDanger].filter(Boolean).join(' ')}
              disabled={busy}
              onClick={() => {
                if (trashArmed) runBatch('trash')
                else {
                  setPurgeArmed(false)
                  setTrashArmed(true)
                }
              }}
            >
              {trashArmed ? t('adminA.confirmMoveToTrash') : t('adminA.batchMoveToTrash')}
            </button>
          )}
          <button
            type="button"
            className={[styles.batchAct, purgeArmed && styles.batchDanger].filter(Boolean).join(' ')}
            disabled={busy}
            onClick={() => {
              if (purgeArmed) runBatch('purge')
              else {
                setTrashArmed(false)
                setPurgeArmed(true)
              }
            }}
          >
            {purgeArmed ? t('adminA.confirmPurge') : t('adminA.batchPurge')}
          </button>
          <button type="button" className={styles.batchClose} title={t('adminA.clearSelection')} onClick={() => setSelected(new Set())}>
            ×
          </button>
        </div>
      )}

      <AdminImageDetail item={detail} onClose={() => setDetail(null)} />
    </div>
  )
}
