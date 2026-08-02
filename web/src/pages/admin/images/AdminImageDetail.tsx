import { useDeleteAdminImage, usePurgeAdminImage, useSetImageWhitelist } from '../../../api/adminHooks'
import type { AdminImageDeleteResult, AdminImageItem } from '../../../api/types'
import { useT } from '../../../i18n'
import { copyText } from '../../../lib/copy'
import { formatBytes, formatDate } from '../../../lib/format'
import { useGlobal } from '../../../store'
import { Button } from '../../../ui/Button'
import { InlineConfirm } from '../../../ui/InlineConfirm'
import { Modal } from '../../../ui/Modal'
import styles from './ImagesAdminPage.module.css'

function policyLabel(item: AdminImageItem): string {
  if (item.policy_name) return `${item.policy_name} (#${item.policy_id})`
  if (item.policy_id) return `#${item.policy_id}`
  return '—'
}

function uploaderLabel(item: AdminImageItem, guestLabel: string): string {
  if (item.user_id == null) return guestLabel
  return `${item.username || '—'}(#${item.user_id})`
}

function purgeToast(t: (k: string) => string, res?: AdminImageDeleteResult): string {
  if (!res?.permanent) return t('adminA.toastPurged')
  if (res.object_retained) return t('adminA.toastPurgedRetained')
  if (res.physical_queued) return t('adminA.toastPurgedQueued')
  // permanent but neither queued nor retained: runner missing or enqueue failed
  return t('adminA.toastPurgedNoQueue')
}

export function AdminImageDetail({ item, onClose }: { item: AdminImageItem | null; onClose(): void }) {
  const { t } = useT()
  const pushToast = useGlobal((s) => s.pushToast)
  const wl = useSetImageWhitelist()
  const delM = useDeleteAdminImage()
  const purgeM = usePurgeAdminImage()
  const statusLabel = (status: string) => {
    if (status === 'normal') return t('adminA.statusNormal')
    if (status === 'pending') return t('adminA.statusPending')
    if (status === 'rejected') return t('adminA.statusRejected')
    return status
  }
  const isGuest = item?.user_id == null
  const inTrash = !!item?.in_trash
  // 游客无回收站；已在回收站只能彻底删除
  const showSoftTrash = !isGuest && !inTrash
  const busy = delM.isPending || purgeM.isPending || wl.isPending

  return (
    <Modal open={item !== null} onClose={onClose} width={560}>
      {item && (
        <div className={styles.detail}>
          <img className={styles.detailImg} src={item.links.thumbnail_url} alt={item.name} />
          <div className={styles.detailMeta}>
            <div className={styles.detailName}>{item.name}</div>
            <dl className={styles.detailList}>
              <dt>{t('adminA.uploader')}</dt>
              <dd>{uploaderLabel(item, t('adminA.guestUploader'))}</dd>
              <dt>{t('adminA.size')}</dt>
              <dd>{formatBytes(item.size)}</dd>
              <dt>{t('adminA.status')}</dt>
              <dd>
                {statusLabel(item.status)}
                {item.is_whitelisted && t('adminA.whitelistedSuffix')}
                {inTrash && ` · ${t('adminA.trashBadge')}`}
              </dd>
              <dt>NSFW</dt>
              <dd>{item.nsfw_score == null ? '—' : String(item.nsfw_score)}</dd>
              <dt>{t('adminA.storagePolicy')}</dt>
              <dd>{policyLabel(item)}</dd>
              <dt>{t('adminA.storageDriver')}</dt>
              <dd>{item.policy_driver || '—'}</dd>
              <dt>{t('adminA.storageSurface')}</dt>
              <dd>{item.surface || '—'}</dd>
              <dt>{t('adminA.storagePath')}</dt>
              <dd className={styles.detailPath}>
                <code>{item.path || '—'}</code>
                {item.path ? (
                  <Button variant="secondary" onClick={() => copyText(item.path, t('adminA.storagePath'))}>
                    {t('adminA.copy')}
                  </Button>
                ) : null}
              </dd>
              <dt>{t('adminA.uploadedAt')}</dt>
              <dd>{formatDate(item.created_at)}</dd>
            </dl>
            <div className={styles.detailUrl}>
              <span>{item.links.url}</span>
              <Button variant="secondary" onClick={() => copyText(item.links.url, t('adminA.urlLabel'))}>
                {t('adminA.copy')}
              </Button>
            </div>
            <p className={styles.detailHint}>{t('adminA.deleteHint')}</p>
            <div className={styles.detailBtns}>
              {!inTrash && (
                <Button
                  variant="secondary"
                  disabled={busy}
                  onClick={() => wl.mutate({ key: item.key, on: !item.is_whitelisted }, { onSuccess: onClose })}
                >
                  {item.is_whitelisted ? t('adminA.unwhitelist') : t('adminA.whitelist')}
                </Button>
              )}
              {showSoftTrash && (
                <InlineConfirm
                  label={t('adminA.moveToTrash')}
                  confirmLabel={t('adminA.confirmMoveToTrash')}
                  disabled={busy}
                  onConfirm={() =>
                    delM.mutate(item.key, {
                      onSuccess: (res) => {
                        pushToast(res.permanent ? purgeToast(t, res) : t('adminA.toastMovedToTrash'))
                        onClose()
                      },
                    })
                  }
                />
              )}
              <InlineConfirm
                label={t('adminA.purgePermanent')}
                confirmLabel={t('adminA.confirmPurge')}
                disabled={busy}
                onConfirm={() =>
                  purgeM.mutate(item.key, {
                    onSuccess: (res) => {
                      pushToast(purgeToast(t, res))
                      onClose()
                    },
                  })
                }
              />
            </div>
          </div>
        </div>
      )}
    </Modal>
  )
}
