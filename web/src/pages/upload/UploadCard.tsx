import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { copyText } from '../../lib/copy'
import { formatBytes } from '../../lib/format'
import type { QueueItem } from '../../upload/queue'
import { useUploadQueue } from '../../upload/queue'

const BADGE_CLS: Record<QueueItem['status'], string> = {
  queued: 'text-muted',
  uploading: 'text-ink',
  processing: 'bg-soft text-ink',
  success: 'text-ok',
  instant: 'border-btn bg-btn text-btn-text',
  failed: 'border-err text-err',
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
    <div
      className={cn(
        'animate-[rise_0.25s_both] rounded-sm border border-border bg-surface p-3 px-3.5',
        item.status === 'failed' && 'border-err',
        done && 'border-[color-mix(in_srgb,var(--ok)_35%,var(--border))]',
      )}
    >
      <div className="flex items-center gap-3.5">
        {item.thumb ? (
          <div
            className="size-12 shrink-0 rounded-[2px] bg-soft bg-cover bg-center"
            style={{ backgroundImage: `url(${item.thumb})` }}
          />
        ) : (
          <div className="stripe flex size-12 shrink-0 items-center justify-center rounded-[2px] border border-border font-mono text-[9px] text-muted">
            {(item.ext || 'img').toUpperCase().slice(0, 4)}
          </div>
        )}
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <div className="flex items-baseline gap-2.5">
            <span className="truncate text-[13.5px] font-bold">{item.name}</span>
            {item.size > 0 && <span className="shrink-0 font-mono text-xs-plus text-muted">{formatBytes(item.size)}</span>}
            <span
              className={cn(
                'shrink-0 rounded-[2px] border border-border px-[7px] py-px font-mono text-[9.5px] tracking-[0.08em]',
                badgeCls,
              )}
            >
              {badgeLabel}
            </span>
          </div>
          {(item.status === 'uploading' || item.status === 'queued') && (
            <div className="flex items-center gap-2.5">
              <div className="h-0.5 flex-1 overflow-hidden bg-soft">
                <div className="h-full bg-btn transition-[width] duration-150" style={{ width: `${Math.round(item.pct)}%` }} />
              </div>
              <span className="min-w-[34px] shrink-0 text-right font-mono text-[11px] text-muted">
                {Math.round(item.pct)}%
              </span>
            </div>
          )}
          {subText && (
            <span
              className={cn(
                'truncate font-mono text-[11.5px] text-muted',
                item.status === 'failed' && 'text-err',
              )}
            >
              {subText}
            </span>
          )}
        </div>
        {item.status === 'failed' && item.retryable && (
          <button
            type="button"
            className="shrink-0 cursor-pointer rounded-sm border border-border bg-surface px-3.5 py-1.5 text-[11.5px] font-bold text-ink hover:bg-soft"
            onClick={() => retry(item.id)}
          >
            {t('upload.retry')}
          </button>
        )}
        <button
          type="button"
          className="shrink-0 cursor-pointer border-0 bg-transparent px-1.5 py-1 text-base leading-none text-muted hover:text-ink"
          aria-label={t('upload.remove')}
          title={t('upload.remove')}
          onClick={() => remove(item.id)}
        >
          ×
        </button>
      </div>

      {done && links && (
        <div className="mt-3 flex animate-[fadeIn_0.2s] flex-col gap-2.5 border-t border-border pt-3">
          <div className="flex items-stretch gap-0 overflow-hidden rounded-sm border border-border bg-bg">
            <input
              className="min-w-0 flex-1 border-0 bg-transparent px-3 py-2.5 font-mono text-sm-plus text-ink outline-none max-[560px]:px-2.5 max-[560px]:py-[9px] max-[560px]:text-[11.5px]"
              readOnly
              value={links.url}
              aria-label={t('upload.linkNameUrl')}
              onFocus={(e) => e.currentTarget.select()}
            />
            <button
              type="button"
              className="shrink-0 cursor-pointer whitespace-nowrap border-0 border-l border-border bg-btn px-4 text-sm-plus font-bold text-btn-text hover:brightness-105 max-[560px]:px-3 max-[560px]:text-[11.5px]"
              onClick={() => copyText(links.url, t('upload.linkNameUrl'))}
            >
              {t('upload.copyUrl')}
            </button>
          </div>
          {shareURL && (
            <div className="flex items-stretch gap-0 overflow-hidden rounded-sm border border-border bg-bg">
              <input
                className="min-w-0 flex-1 border-0 bg-transparent px-3 py-2.5 font-mono text-sm-plus text-ink outline-none max-[560px]:px-2.5 max-[560px]:py-[9px] max-[560px]:text-[11.5px]"
                readOnly
                value={shareURL}
                aria-label={t('upload.linkNameShare')}
                onFocus={(e) => e.currentTarget.select()}
              />
              <button
                type="button"
                className="shrink-0 cursor-pointer whitespace-nowrap border-0 border-l border-border bg-surface px-4 text-sm-plus font-bold text-ink hover:bg-soft max-[560px]:px-3 max-[560px]:text-[11.5px]"
                onClick={() => copyText(shareURL, t('upload.linkNameShare'))}
              >
                {t('upload.copyShare')}
              </button>
            </div>
          )}
          {shareURL && <p className="m-0 text-[11px] leading-snug text-muted">{t('upload.shareHint')}</p>}
          <div className="flex flex-wrap items-center justify-between gap-2.5 max-[560px]:flex-col max-[560px]:items-stretch">
            <div
              className="flex shrink-0 overflow-hidden rounded-sm border border-border max-[560px]:w-full max-[560px]:overflow-x-auto"
              role="group"
              aria-label={t('upload.formatsAria')}
            >
              {formats.map((c) => (
                <button
                  key={c.label}
                  type="button"
                  className="cursor-pointer border-0 border-l border-border bg-surface px-[11px] py-1.5 font-mono text-xs-plus font-semibold tracking-[0.04em] text-ink first:border-l-0 hover:bg-soft"
                  onClick={() => copyText(c.text, c.name)}
                >
                  {c.label}
                </button>
              ))}
            </div>
            {shareURL && (
              <a
                className="whitespace-nowrap text-xs font-semibold text-ink underline underline-offset-2 hover:opacity-85 max-[560px]:self-start"
                href={shareURL}
                target="_blank"
                rel="noopener noreferrer"
              >
                {t('upload.openShare')}
              </a>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
