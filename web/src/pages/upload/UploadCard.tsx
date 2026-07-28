import { useT } from '../../i18n'
import { copyText } from '../../lib/copy'
import { formatBytes } from '../../lib/format'
import type { QueueItem } from '../../upload/queue'
import { useUploadQueue } from '../../upload/queue'
import styles from './UploadCard.module.css'

const BADGE_CLS: Record<QueueItem['status'], string> = {
  queued: styles.bQueued,
  uploading: styles.bUploading,
  processing: styles.bProcessing,
  success: styles.bSuccess,
  instant: styles.bInstant,
  failed: styles.bFailed,
}

const STATUS_KEY: Record<QueueItem['status'], string> = {
  queued: 'upload.statusQueued',
  uploading: 'upload.statusUploading',
  processing: 'upload.statusProcessing',
  success: 'upload.statusSuccess',
  instant: 'upload.statusInstant',
  failed: 'upload.statusFailed',
}

export function UploadCard({ item }: { item: QueueItem }) {
  const { t } = useT()
  const retry = useUploadQueue((s) => s.retry)
  const remove = useUploadQueue((s) => s.remove)
  const done = item.status === 'success' || item.status === 'instant'
  const badgeLabel = t(STATUS_KEY[item.status])
  const badgeCls = BADGE_CLS[item.status]
  const links = item.result?.links
  const subText =
    item.status === 'failed'
      ? item.reason
      : item.status === 'instant'
        ? t('upload.instantHint')
        : item.status === 'processing'
          ? t('upload.processingHint')
          : null

  const copyRow = links
    ? [
        { label: 'URL', text: links.url, name: t('upload.linkNameUrl') },
        { label: 'MD', text: links.markdown, name: t('upload.linkNameMd') },
        { label: 'HTML', text: links.html, name: t('upload.linkNameHtml') },
        { label: 'BB', text: links.bbcode, name: t('upload.linkNameBb') },
        { label: t('upload.thumbnail'), text: links.thumbnail_url, name: t('upload.linkNameThumb') },
        {
          label: t('upload.copyAll'),
          text: [links.url, links.markdown, links.html, links.bbcode].join('\n'),
          name: t('upload.linkNameAll'),
        },
      ]
    : []

  return (
    <div className={[styles.card, item.status === 'failed' && styles.cardFailed].filter(Boolean).join(' ')}>
      <div className={styles.row}>
        {item.thumb ? (
          <div className={styles.thumb} style={{ backgroundImage: `url(${item.thumb})` }} />
        ) : (
          <div className={`stripe ${styles.thumbPh}`}>{(item.ext || 'img').toUpperCase().slice(0, 4)}</div>
        )}
        <div className={styles.mid}>
          <div className={styles.titleRow}>
            <span className={styles.name}>{item.name}</span>
            {item.size > 0 && <span className={styles.size}>{formatBytes(item.size)}</span>}
            <span className={`${styles.badge} ${badgeCls}`}>{badgeLabel}</span>
          </div>
          {(item.status === 'uploading' || item.status === 'queued') && (
            <div className={styles.barRow}>
              <div className={styles.track}>
                <div className={styles.fill} style={{ width: `${Math.round(item.pct)}%` }} />
              </div>
              <span className={styles.pct}>{Math.round(item.pct)}%</span>
            </div>
          )}
          {subText && (
            <span className={[styles.sub, item.status === 'failed' && styles.subErr].filter(Boolean).join(' ')}>
              {subText}
            </span>
          )}
        </div>
        {item.status === 'failed' && item.retryable && (
          <button type="button" className={styles.retryBtn} onClick={() => retry(item.id)}>
            {t('upload.retry')}
          </button>
        )}
        <button
          type="button"
          className={styles.removeBtn}
          aria-label={t('upload.remove')}
          title={t('upload.remove')}
          onClick={() => remove(item.id)}
        >
          ×
        </button>
      </div>
      {done && links && (
        <div className={styles.linksRow}>
          <div className={styles.copyGroup}>
            {copyRow.map((c) => (
              <button key={c.label} type="button" className={styles.copyBtn} onClick={() => copyText(c.text, c.name)}>
                {c.label}
              </button>
            ))}
          </div>
          <span className={styles.urlText}>{links.url}</span>
        </div>
      )}
    </div>
  )
}
