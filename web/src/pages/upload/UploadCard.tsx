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

function sharePageURL(item: QueueItem): string | null {
  const links = item.result?.links
  if (!links) return null
  // Private uploads have no public share page
  if (item.opts.visibility === 'private') return null
  if (links.share_url) return links.share_url
  const key = item.result?.key
  if (!key || typeof window === 'undefined') return null
  return `${window.location.origin}/s/${key}`
}

export function UploadCard({ item }: { item: QueueItem }) {
  const { t } = useT()
  const retry = useUploadQueue((s) => s.retry)
  const remove = useUploadQueue((s) => s.remove)
  const done = item.status === 'success' || item.status === 'instant'
  const badgeLabel = t(STATUS_KEY[item.status])
  const badgeCls = BADGE_CLS[item.status]
  const links = item.result?.links
  const shareURL = done ? sharePageURL(item) : null
  const subText =
    item.status === 'failed'
      ? item.reason
      : item.status === 'instant'
        ? item.result?.reused
          ? t('upload.reusedHint')
          : t('upload.instantHint')
        : item.status === 'processing'
          ? t('upload.processingHint')
          : null

  const formats = links
    ? [
        { label: 'MD', text: links.markdown, name: t('upload.linkNameMd') },
        { label: 'HTML', text: links.html, name: t('upload.linkNameHtml') },
        { label: 'BB', text: links.bbcode, name: t('upload.linkNameBb') },
        { label: t('upload.thumbnail'), text: links.thumbnail_url, name: t('upload.linkNameThumb') },
        {
          label: t('upload.copyAll'),
          text: [links.url, links.markdown, links.html, links.bbcode, shareURL].filter(Boolean).join('\n'),
          name: t('upload.linkNameAll'),
        },
      ]
    : []

  return (
    <div className={[styles.card, item.status === 'failed' && styles.cardFailed, done && styles.cardDone].filter(Boolean).join(' ')}>
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
        <div className={styles.successPanel}>
          <div className={styles.primaryRow}>
            <input
              className={styles.primaryUrl}
              readOnly
              value={links.url}
              aria-label={t('upload.linkNameUrl')}
              onFocus={(e) => e.currentTarget.select()}
            />
            <button
              type="button"
              className={styles.primaryCopy}
              onClick={() => copyText(links.url, t('upload.linkNameUrl'))}
            >
              {t('upload.copyUrl')}
            </button>
          </div>
          {shareURL && (
            <div className={styles.primaryRow}>
              <input
                className={styles.primaryUrl}
                readOnly
                value={shareURL}
                aria-label={t('upload.linkNameShare')}
                onFocus={(e) => e.currentTarget.select()}
              />
              <button
                type="button"
                className={styles.shareCopy}
                onClick={() => copyText(shareURL, t('upload.linkNameShare'))}
              >
                {t('upload.copyShare')}
              </button>
            </div>
          )}
          {shareURL && <p className={styles.shareHint}>{t('upload.shareHint')}</p>}
          <div className={styles.formatsRow}>
            <div className={styles.copyGroup} role="group" aria-label={t('upload.formatsAria')}>
              {formats.map((c) => (
                <button key={c.label} type="button" className={styles.copyBtn} onClick={() => copyText(c.text, c.name)}>
                  {c.label}
                </button>
              ))}
            </div>
            {shareURL && (
              <a className={styles.shareLink} href={shareURL} target="_blank" rel="noopener noreferrer">
                {t('upload.openShare')}
              </a>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
