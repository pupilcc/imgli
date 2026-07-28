import { useDeleteAdminImage, useSetImageWhitelist } from '../../../api/adminHooks'
import type { AdminImageItem } from '../../../api/types'
import { useT } from '../../../i18n'
import { copyText } from '../../../lib/copy'
import { formatBytes, formatDate } from '../../../lib/format'
import { Button } from '../../../ui/Button'
import { InlineConfirm } from '../../../ui/InlineConfirm'
import { Modal } from '../../../ui/Modal'
import styles from './ImagesAdminPage.module.css'

export function AdminImageDetail({ item, onClose }: { item: AdminImageItem | null; onClose(): void }) {
  const { t } = useT()
  const wl = useSetImageWhitelist()
  const delM = useDeleteAdminImage()
  const statusLabel = (status: string) => {
    if (status === 'normal') return t('adminA.statusNormal')
    if (status === 'pending') return t('adminA.statusPending')
    if (status === 'rejected') return t('adminA.statusRejected')
    return status
  }
  return (
    <Modal open={item !== null} onClose={onClose} width={560}>
      {item && (
        <div className={styles.detail}>
          <img className={styles.detailImg} src={item.links.thumbnail_url} alt={item.name} />
          <div className={styles.detailMeta}>
            <div className={styles.detailName}>{item.name}</div>
            <dl className={styles.detailList}>
              <dt>{t('adminA.uploader')}</dt>
              <dd>{item.username}(#{item.user_id})</dd>
              <dt>{t('adminA.size')}</dt>
              <dd>{formatBytes(item.size)}</dd>
              <dt>{t('adminA.status')}</dt>
              <dd>{statusLabel(item.status)}{item.is_whitelisted && t('adminA.whitelistedSuffix')}</dd>
              <dt>NSFW</dt>
              <dd>{item.nsfw_score == null ? '—' : String(item.nsfw_score)}</dd>
              <dt>{t('adminA.uploadedAt')}</dt>
              <dd>{formatDate(item.created_at)}</dd>
            </dl>
            <div className={styles.detailUrl}>
              <span>{item.links.url}</span>
              <Button variant="secondary" onClick={() => copyText(item.links.url, t('adminA.urlLabel'))}>
                {t('adminA.copy')}
              </Button>
            </div>
            <div className={styles.detailBtns}>
              <Button
                variant="secondary"
                disabled={wl.isPending}
                onClick={() => wl.mutate({ key: item.key, on: !item.is_whitelisted }, { onSuccess: onClose })}
              >
                {item.is_whitelisted ? t('adminA.unwhitelist') : t('adminA.whitelist')}
              </Button>
              <InlineConfirm
                label={t('common.delete')}
                onConfirm={() => delM.mutate(item.key, { onSuccess: onClose })}
              />
            </div>
          </div>
        </div>
      )}
    </Modal>
  )
}
