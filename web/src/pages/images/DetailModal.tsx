import { useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'
import { renderSVG } from 'uqr'
import { useAlbums, useDeleteImage, useImageDetail, useImageStats, useUpdateImage } from '../../api/hooks'
import type { ImageItem } from '../../api/types'
import { useT } from '../../i18n'
import { copyText } from '../../lib/copy'
import { formatBytes, formatDate } from '../../lib/format'
import { useGlobal } from '../../store'
import { InlineConfirm } from '../../ui/InlineConfirm'
import { Segmented } from '../../ui/Segmented'
import styles from './DetailModal.module.css'

/** value=expires_in 秒;0=永久 */
const EXPIRY_PRESETS = [
  { key: 'never', sec: 0 },
  { key: '1h', sec: 3600 },
  { key: '1d', sec: 86400 },
  { key: '7d', sec: 604800 },
  { key: '30d', sec: 2592000 },
] as const

type ExpiryKey = (typeof EXPIRY_PRESETS)[number]['key']

const EXPIRY_LABEL_KEY: Record<ExpiryKey, string> = {
  never: 'upload.expiryNever',
  '1h': 'upload.expiry1h',
  '1d': 'upload.expiry1d',
  '7d': 'upload.expiry7d',
  '30d': 'upload.expiry30d',
}

const MAX_VIEWS_PRESETS = [
  { key: 'unlimited', n: 0 },
  { key: '1', n: 1 },
  { key: '3', n: 3 },
  { key: '10', n: 10 },
] as const
type MaxViewsKey = (typeof MAX_VIEWS_PRESETS)[number]['key']
const MAX_VIEWS_LABEL_KEY: Record<MaxViewsKey, string> = {
  unlimited: 'upload.maxViewsUnlimited',
  '1': 'upload.maxViews1',
  '3': 'upload.maxViews3',
  '10': 'upload.maxViews10',
}

interface Props {
  items: ImageItem[]
  focusKey: string
  onClose(): void
  onNavigate(key: string): void
}

export function DetailModal({ items, focusKey, onClose, onNavigate }: Props) {
  const { t, lang } = useT()
  const idx = items.findIndex((i) => i.key === focusKey)
  const base = items[idx]
  const detail = useImageDetail(focusKey)
  const stats = useImageStats(focusKey)
  const albums = useAlbums()
  const update = useUpdateImage()
  const remove = useDeleteImage()
  const pushToast = useGlobal((s) => s.pushToast)
  const [renaming, setRenaming] = useState(false)
  const [renameVal, setRenameVal] = useState('')
  const [moving, setMoving] = useState(false)
  const [expiryKey, setExpiryKey] = useState<ExpiryKey>('never')

  const prevKey = idx > 0 ? items[idx - 1].key : null
  const nextKey = idx >= 0 && idx < items.length - 1 ? items[idx + 1].key : null

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = document.activeElement
      const editing = el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement
      if (e.key === 'Escape') {
        onClose()
        return
      }
      if (editing) return
      if (e.key === 'ArrowLeft' && prevKey) onNavigate(prevKey)
      if (e.key === 'ArrowRight' && nextKey) onNavigate(nextKey)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [prevKey, nextKey, onClose, onNavigate])

  // 切换图片时复位内联编辑
  useEffect(() => {
    setRenaming(false)
    setMoving(false)
    setExpiryKey('never')
  }, [focusKey])

  const qrUrl = base?.links.url ?? ''
  const qrSVG = useMemo(() => (qrUrl ? renderSVG(qrUrl) : ''), [qrUrl])

  if (!base) return null
  const d = detail.data
  const albumName = base.album_id
    ? (albums.data?.items.find((a) => a.id === base.album_id)?.name ?? `#${base.album_id}`)
    : t('images.uncategorized')
  // detail 可能在 PATCH 后先刷新;列表项次之
  const expiresAt = d?.expires_at !== undefined ? d.expires_at : base.expires_at
  const expiryDisplay = (() => {
    if (!expiresAt) return t('images.permanent')
    const date = new Date(expiresAt).toLocaleDateString(lang === 'zh' ? 'zh-CN' : 'en-US')
    return t('images.expiresOn', { date })
  })()

  const commitRename = () => {
    const name = renameVal.trim()
    if (!name || name === base.name) return setRenaming(false)
    update.mutate({ key: base.key, body: { name } }, {
      onSuccess: () => {
        setRenaming(false)
        pushToast(t('images.renamed'))
      },
    })
  }

  const setExpiry = (sec: number) => {
    update.mutate({ key: base.key, body: { expires_in: sec } })
  }

  const maxViews = d?.max_views ?? base.max_views ?? 0
  const viewsServed = d?.views_served ?? base.views_served ?? 0
  const maxViewsKey: MaxViewsKey =
    MAX_VIEWS_PRESETS.find((p) => p.n === maxViews)?.key ?? 'unlimited'
  const setMaxViews = (n: number) => {
    update.mutate({ key: base.key, body: { max_views: n } })
  }

  const hasAccessPassword = !!(d?.has_access_password ?? base.has_access_password)
  const [accessPw, setAccessPw] = useState('')
  const setAccessPassword = (password: string) => {
    update.mutate(
      { key: base.key, body: { access_password: password } },
      { onSuccess: () => setAccessPw('') },
    )
  }

  const copyRows = [
    { kind: 'URL', text: base.links.url },
    { kind: 'MD', text: base.links.markdown },
    { kind: 'HTML', text: base.links.html },
    { kind: 'BBCODE', text: base.links.bbcode },
    { kind: t('images.thumbnail'), text: base.links.thumbnail_url },
  ]

  const visLabel = base.visibility === 'public' ? 'PUBLIC' : 'PRIVATE'

  return createPortal(
    <div className={styles.mask} onClick={onClose}>
      {prevKey && (
        <button type="button" title={t('images.prev')} className={`${styles.navBtn} ${styles.navPrev}`} onClick={(e) => { e.stopPropagation(); onNavigate(prevKey) }}>
          ‹
        </button>
      )}
      {nextKey && (
        <button type="button" title={t('images.next')} className={`${styles.navBtn} ${styles.navNext}`} onClick={(e) => { e.stopPropagation(); onNavigate(nextKey) }}>
          ›
        </button>
      )}
      <div role="dialog" className={styles.box} onClick={(e) => e.stopPropagation()}>
        <div className={styles.handle} />
        <div className={styles.preview}>
          <img className={styles.previewImg} src={base.links.url} alt={base.name} />
          <span className={styles.pos}>{idx + 1} / {items.length}</span>
        </div>
        <div className={styles.pane}>
          <div className={styles.headRow}>
            <div className={styles.headLeft}>
              <div className={styles.kicker}>DETAIL</div>
              {renaming ? (
                <div className={styles.renameRow}>
                  <input
                    className={styles.renameInput}
                    autoFocus
                    value={renameVal}
                    onChange={(e) => setRenameVal(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') commitRename()
                    }}
                  />
                  <button type="button" className={styles.renameSave} onClick={commitRename}>{t('images.save')}</button>
                </div>
              ) : (
                <div className={styles.nameRow}>
                  <span className={styles.name}>{base.name}</span>
                  <button
                    type="button"
                    className={styles.renameLink}
                    onClick={() => {
                      setRenameVal(base.name)
                      setRenaming(true)
                    }}
                  >
                    {t('images.rename')}
                  </button>
                </div>
              )}
            </div>
            <button type="button" className={styles.closeBtn} onClick={onClose}>×</button>
          </div>

          <div className={styles.metaTable}>
            <span className={styles.metaKey}>{t('images.dims')}</span><span className={styles.metaVal}>{base.width} × {base.height}</span>
            <span className={styles.metaKey}>{t('images.size')}</span><span className={styles.metaVal}>{formatBytes(base.size)}</span>
            <span className={styles.metaKey}>MIME</span><span className={styles.metaVal}>{d?.mime ?? '…'}</span>
            <span className={styles.metaKey}>{t('images.uploadedAt')}</span><span className={styles.metaVal}>{formatDate(base.created_at)}</span>
            <span className={styles.metaKey}>{t('images.uploadIp')}</span>
            <span className={`${styles.metaVal} ${styles.metaMuted}`}>{d ? t('images.ipSelfOnly', { ip: d.upload_ip || '—' }) : '…'}</span>
            <span className={styles.metaKey}>{t('images.album')}</span><span className={styles.metaVal}>{albumName}</span>
            <span className={styles.metaKey}>{t('images.visibility')}</span>
            <button
              type="button"
              className={styles.visToggle}
              disabled={update.isPending}
              onClick={() => update.mutate({ key: base.key, body: { visibility: base.visibility === 'public' ? 'private' : 'public' } })}
            >
              {visLabel} — {t('images.clickToToggle')}
            </button>
            <span className={styles.metaKey}>{t('images.slug')}</span>
            <span className={styles.metaVal}>
              <input
                className={styles.renameInput}
                defaultValue={base.slug ?? ''}
                placeholder="my-photo"
                onBlur={(e) => {
                  const v = e.target.value.trim().toLowerCase()
                  const cur = (base.slug ?? '').toLowerCase()
                  if (v === cur) return
                  update.mutate({ key: base.key, body: { slug: v } })
                }}
              />
            </span>
            <span className={styles.metaKey}>{t('images.expiry')}</span>
            <span className={styles.metaVal}>{expiryDisplay}</span>
          </div>

          <div className={styles.expiryEdit}>
            <Segmented<ExpiryKey>
              mono
              options={EXPIRY_PRESETS.map((p) => ({
                value: p.key,
                label: t(EXPIRY_LABEL_KEY[p.key]),
              }))}
              value={expiryKey}
              onChange={(k) => {
                setExpiryKey(k)
                const p = EXPIRY_PRESETS.find((x) => x.key === k)
                setExpiry(p?.sec ?? 0)
              }}
            />
            {expiresAt && (
              <button
                type="button"
                className={styles.removeExpiry}
                disabled={update.isPending}
                onClick={() => {
                  setExpiryKey('never')
                  setExpiry(0)
                }}
              >
                {t('images.removeExpiry')}
              </button>
            )}
            <div className={styles.expiryWarn}>{t('images.expiryWarn')}</div>
          </div>

          <div className={styles.metaTable}>
            <span className={styles.metaKey}>{t('images.maxViews')}</span>
            <span className={styles.metaVal}>
              {maxViews > 0
                ? t('images.maxViewsUsed', { used: viewsServed, max: maxViews })
                : t('upload.maxViewsUnlimited')}
            </span>
          </div>
          <div className={styles.expiryEdit}>
            <Segmented<MaxViewsKey>
              mono
              options={MAX_VIEWS_PRESETS.map((p) => ({
                value: p.key,
                label: t(MAX_VIEWS_LABEL_KEY[p.key]),
              }))}
              value={maxViewsKey}
              onChange={(k) => {
                const p = MAX_VIEWS_PRESETS.find((x) => x.key === k)
                setMaxViews(p?.n ?? 0)
              }}
            />
            <div className={styles.expiryWarn}>{t('images.maxViewsHint')}</div>
          </div>

          <div className={styles.metaTable}>
            <span className={styles.metaKey}>{t('images.accessPassword')}</span>
            <span className={styles.metaVal}>
              {hasAccessPassword ? t('images.accessPasswordSet') : t('images.accessPasswordNone')}
            </span>
          </div>
          <div className={styles.expiryEdit}>
            <input
              className={styles.renameInput}
              type="password"
              value={accessPw}
              placeholder={t('images.accessPasswordPlaceholder')}
              onChange={(e) => setAccessPw(e.target.value)}
              autoComplete="new-password"
            />
            <div className={styles.renameRow}>
              <button
                type="button"
                className={styles.renameSave}
                disabled={!accessPw.trim() || update.isPending}
                onClick={() => setAccessPassword(accessPw.trim())}
              >
                {t('images.accessPasswordSave')}
              </button>
              {hasAccessPassword && (
                <button
                  type="button"
                  className={styles.removeExpiry}
                  disabled={update.isPending}
                  onClick={() => setAccessPassword('')}
                >
                  {t('images.accessPasswordClear')}
                </button>
              )}
            </div>
            <div className={styles.expiryWarn}>{t('images.accessPasswordHint')}</div>
          </div>

          <div className={styles.copySection}>
            <div className={styles.copyCol}>
              <div className={styles.kicker}>COPY LINK</div>
              {copyRows.map((r) => (
                <div key={r.kind} className={styles.copyRow}>
                  <span className={styles.copyKind}>{r.kind}</span>
                  <span className={styles.copyText}>{r.text}</span>
                  <button type="button" className={styles.copyBtn} onClick={() => copyText(r.text, t('images.linkLabel', { kind: r.kind }))}>
                    {t('images.copy')}
                  </button>
                </div>
              ))}
            </div>
            <div className={styles.qrCol}>
              <div className={styles.kicker}>QR</div>
              <div className={styles.qrBox} dangerouslySetInnerHTML={{ __html: qrSVG }} />
            </div>
          </div>

          {stats.isError || !stats.data ? null : (
            <div className={styles.accessSection}>
              <div className={styles.kicker}>ACCESS — {t('images.accessLabel')}</div>
              <div className={styles.accessTotal}>{t('images.totalViews', { count: stats.data.total })}</div>
              {(() => {
                const daily = stats.data.daily
                const max = daily.reduce((m, day) => (day.views > m ? day.views : m), 0)
                if (max === 0) return <div className={styles.accessEmpty}>{t('images.noAccess')}</div>
                return (
                  <div className={styles.accessBars} aria-label={t('images.last30Days')}>
                    {daily.map((day) => (
                      <div
                        key={day.date}
                        className={styles.accessBar}
                        title={`${day.date}: ${day.views}`}
                        style={{ height: day.views > 0 ? `${Math.max(4, Math.round((day.views / max) * 100))}%` : '0%' }}
                      />
                    ))}
                  </div>
                )
              })()}
            </div>
          )}

          <div className={styles.footRow}>
            {moving ? (
              <select
                className={styles.moveSelect}
                aria-label={t('images.moveToAlbum')}
                autoFocus
                disabled={update.isPending}
                defaultValue={String(base.album_id ?? 'none')}
                onChange={(e) => {
                  const v = e.target.value
                  update.mutate(
                    { key: base.key, body: { album_id: v === 'none' ? null : Number(v) } },
                    { onSuccess: () => { setMoving(false); pushToast(t('images.moved')) } },
                  )
                }}
              >
                <option value="none">{t('images.uncategorized')}</option>
                {albums.data?.items.map((a) => (
                  <option key={a.id} value={a.id}>{a.name}</option>
                ))}
              </select>
            ) : (
              <button type="button" className={styles.footBtn} onClick={() => setMoving(true)}>{t('images.moveToAlbum')}</button>
            )}
            <InlineConfirm
              label={t('images.addToTrash')}
              disabled={remove.isPending}
              onConfirm={() =>
                remove.mutate(base.key, {
                  onSuccess: () => {
                    pushToast(t('images.trashed'))
                    onClose()
                  },
                })
              }
            />
          </div>
        </div>
      </div>
    </div>,
    document.body,
  )
}
