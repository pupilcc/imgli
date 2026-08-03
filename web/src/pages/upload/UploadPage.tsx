import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { useQueryClient } from '@tanstack/react-query'
import { useAlbums, useConfig, useQuota, useSession, useUserPolicies } from '../../api/hooks'
import { useT } from '../../i18n'
import { formatBytes } from '../../lib/format'
import {
  EXPIRY_PRESETS,
  expiryPresetLabel,
  filterExpiryPresets,
  filterMaxViewsPresets,
  groupExpiresCapSec,
  maxViewsPresetLabel,
  resolveDefaultExpiresIn,
  resolveDefaultMaxViews,
} from '../../lib/imageAccessPresets'
import { loginHref } from '../../lib/safeNext'
import { useGlobal } from '../../store'
import { extLabel, useUploadQueue, type QueueOpts } from '../../upload/queue'
import { copyText } from '../../lib/copy'
import { QuotaBar, quotaLevel } from '../../ui/QuotaBar'
import { Segmented } from '../../ui/Segmented'
import { Skeleton } from '../../ui/Skeleton'
import { PageHeader } from '../../shell/PageHeader'
import { UploadCard } from './UploadCard'
import styles from './UploadPage.module.css'
import { FirstRunOnboarding } from './FirstRunOnboarding'

const URL_RE = /^https?:\/\/\S+$/

/** Re-export for tests / callers that imported presets from this page. */
export { EXPIRY_PRESETS }

export function UploadPage() {
  const { t } = useT()
  const { data: me } = useSession()
  const isGuest = !me
  const quota = useQuota(!isGuest)
  const albums = useAlbums(!isGuest)
  const policies = useUserPolicies(!isGuest)
  const config = useConfig()
  const qc = useQueryClient()
  const pushToast = useGlobal((s) => s.pushToast)
  const items = useUploadQueue((s) => s.items)
  const addFiles = useUploadQueue((s) => s.addFiles)
  const addUrl = useUploadQueue((s) => s.addUrl)
  const clearDone = useUploadQueue((s) => s.clearDone)

  const prefs = me?.preferences
  const [searchParams] = useSearchParams()
  const albumFromUrl = Number(searchParams.get('album')) || 0
  const [drag, setDrag] = useState(false)
  const [pageDrag, setPageDrag] = useState(false)
  const [fetchUrl, setFetchUrl] = useState('')
  const [optsOpen, setOptsOpen] = useState(false)
  const [visibility, setVisibility] = useState<'public' | 'private'>(() =>
    prefs?.default_visibility === 'private' ? 'private' : 'public',
  )
  const [albumId, setAlbumId] = useState<number | null>(() =>
    albumFromUrl > 0 ? albumFromUrl : (prefs?.default_album_id ?? null),
  )
  // 相册页「上传到此」带 ?album=
  useEffect(() => {
    if (albumFromUrl > 0) setAlbumId(albumFromUrl)
  }, [albumFromUrl])
  const [policyId, setPolicyId] = useState<number | null>(() => prefs?.default_policy_id ?? null)
  const [expiresIn, setExpiresIn] = useState(0)
  const [maxViews, setMaxViews] = useState(0)
  const accessDefaultsSeeded = useRef(false)
  const fileInput = useRef<HTMLInputElement>(null)
  // 队列是全局 store:挂载时把已完成项播种为「已复制」,防路由往返后重复自动复制历史项(codex 终审)
  const copiedRef = useRef<Set<number>>(new Set())
  const copySeeded = useRef(false)
  if (!copySeeded.current) {
    copySeeded.current = true
    for (const i of items) {
      if (i.status === 'success' || i.status === 'instant') copiedRef.current.add(i.id)
    }
  }

  const full = !isGuest && quota.data ? quotaLevel(quota.data.used, quota.data.total) === 'full' : false
  const bwQuota = quota.data?.bandwidth_quota_month ?? 0
  const bwUsed = quota.data?.bandwidth_used_month ?? 0
  const bwFull = !isGuest && bwQuota > 0 && quotaLevel(bwUsed, bwQuota) === 'full'
  const guestUploadOn = !!config.data?.guest_upload_enabled
  const guestLimits = config.data?.guest
  /** 未登录且站点关闭游客上传：展示落地页，不可实际上传 */
  const needLogin = isGuest && config.data != null && !guestUploadOn
  const loginTo = loginHref('/')
  const limits = isGuest
    ? guestUploadOn && guestLimits
      ? { maxFileSize: guestLimits.max_file_size, allowedExts: guestLimits.allowed_exts ?? [] }
      : null
    : quota.data
      ? { maxFileSize: quota.data.max_file_size, allowedExts: quota.data.allowed_exts ?? [] }
      : null
  const showPolicy = (policies.data?.length ?? 0) > 1
  const accessPolicy = isGuest
    ? {
        default_expires_in: guestLimits?.default_expires_in ?? 0,
        max_expires_in: guestLimits?.max_expires_in ?? 0,
        default_max_views: guestLimits?.default_max_views ?? 0,
        max_max_views: guestLimits?.max_max_views ?? 0,
        force_max_age_days: guestLimits?.force_max_age_days ?? 0,
      }
    : {
        default_expires_in: quota.data?.default_expires_in ?? 0,
        max_expires_in: quota.data?.max_expires_in ?? 0,
        default_max_views: quota.data?.default_max_views ?? 0,
        max_max_views: quota.data?.max_max_views ?? 0,
        force_max_age_days: quota.data?.force_max_age_days ?? 0,
      }
  const expiresCap = groupExpiresCapSec(accessPolicy)
  const expiryPresets = filterExpiryPresets(expiresCap)
  const maxViewsPresets = filterMaxViewsPresets(accessPolicy.max_max_views)
  // 组策略就绪后预填默认（仅一次，避免覆盖用户已改选项）
  useEffect(() => {
    if (accessDefaultsSeeded.current) return
    const ready = isGuest ? config.data != null : quota.data != null
    if (!ready) return
    accessDefaultsSeeded.current = true
    setExpiresIn(
      resolveDefaultExpiresIn(
        accessPolicy.default_expires_in,
        expiresCap,
        expiryPresets,
      ),
    )
    setMaxViews(
      resolveDefaultMaxViews(
        accessPolicy.default_max_views,
        accessPolicy.max_max_views,
        maxViewsPresets,
      ),
    )
  }, [isGuest, config.data, quota.data, accessPolicy, expiresCap, expiryPresets, maxViewsPresets])

  const opts: QueueOpts = isGuest
    ? { visibility: 'public', albumId: null, policyId: null, expiresIn, maxViews }
    : { visibility, albumId, policyId: showPolicy ? policyId : null, expiresIn, maxViews }
  const expiryKey =
    expiryPresets.find((p) => p.sec === expiresIn)?.key
    ?? (expiresCap > 0 ? (expiryPresets[0]?.key ?? 'never') : 'never')
  const maxViewsKey =
    maxViewsPresets.find((p) => p.n === maxViews)?.key
    ?? (accessPolicy.max_max_views > 0 ? (maxViewsPresets[0]?.key ?? 'unlimited') : 'unlimited')
  const expiryLabel = expiryPresetLabel(
    expiryPresets.find((p) => p.key === expiryKey) ?? { key: expiryKey, sec: expiresIn },
    t,
  )
  const maxViewsLabel = maxViewsPresetLabel(
    maxViewsPresets.find((p) => p.key === maxViewsKey) ?? { key: maxViewsKey, n: maxViews },
    t,
  )

  useEffect(() => {
    if (albumId !== null && albums.data && !albums.data.items.some((a) => a.id === albumId)) {
      setAlbumId(null)
    }
  }, [albums.data, albumId])
  useEffect(() => {
    if (policyId !== null && policies.data && !policies.data.some((po) => po.id === policyId)) {
      setPolicyId(null)
    }
  }, [policies.data, policyId])

  // 每个完成项让配额失效刷新（导航容量条/警告条实时跟进）
  const doneCount = items.filter((i) => i.status === 'success' || i.status === 'instant').length
  useEffect(() => {
    if (doneCount > 0) qc.invalidateQueries({ queryKey: ['quota'] })
  }, [doneCount, qc])

  const copyFmt = isGuest ? '' : (me?.preferences?.auto_copy_format ?? '')
  useEffect(() => {
    if (!copyFmt) return
    const active = items.some(
      (i) => i.status === 'queued' || i.status === 'uploading' || i.status === 'processing',
    )
    if (active) return
    const fresh = items.filter(
      (i) => (i.status === 'success' || i.status === 'instant') && i.result && !copiedRef.current.has(i.id),
    )
    if (!fresh.length) return
    for (const i of fresh) copiedRef.current.add(i.id)
    const text = fresh
      .map((i) => {
        const links = i.result!.links
        if (copyFmt === 'share') return links.share_url || links.url || ''
        if (copyFmt === 'url') return links.url
        if (copyFmt === 'markdown') return links.markdown
        if (copyFmt === 'html') return links.html
        if (copyFmt === 'bbcode') return links.bbcode
        return links.url
      })
      .filter(Boolean)
      .join('\n')
    navigator.clipboard
      .writeText(text)
      .then(() => pushToast(t('upload.toastAutoCopied', { count: fresh.length })))
      .catch(() => pushToast(t('upload.toastAutoCopyFailed')))
  }, [items, copyFmt, pushToast, t])

  // Ctrl+V 粘贴：监听只挂一次；闸门/选项经 ref 读最新值，避免 opts 每渲染新对象导致重绑。
  const pasteCtx = useRef({ needLogin, full, bwFull, limits, opts, addFiles, pushToast, t })
  pasteCtx.current = { needLogin, full, bwFull, limits, opts, addFiles, pushToast, t }
  useEffect(() => {
    const onPaste = (e: ClipboardEvent) => {
      const ctx = pasteCtx.current
      const imgs = [...(e.clipboardData?.items ?? [])]
        .filter((i) => i.type.startsWith('image/'))
        .map((i) => i.getAsFile())
        .filter((f): f is File => !!f)
      if (!imgs.length) return
      if (ctx.needLogin) return ctx.pushToast(ctx.t('upload.toastLoginRequired'))
      if (ctx.full) return ctx.pushToast(ctx.t('upload.toastQuotaFull'))
      if (ctx.bwFull) return ctx.pushToast(ctx.t('upload.toastBandwidthFull'))
      if (ctx.limits) ctx.addFiles(imgs, ctx.opts, ctx.limits)
    }
    window.addEventListener('paste', onPaste)
    return () => window.removeEventListener('paste', onPaste)
  }, [])

  function acceptFiles(list: FileList | File[]) {
    if (needLogin) return pushToast(t('upload.toastLoginRequired'))
    if (full) return pushToast(t('upload.toastQuotaFull'))
    if (bwFull) return pushToast(t('upload.toastBandwidthFull'))
    if (!limits) return
    const files = [...list].filter((f) => f.type.startsWith('image/') || f.name.includes('.'))
    if (files.length) addFiles(files, opts, limits)
  }

  function doFetch() {
    if (needLogin) return pushToast(t('upload.toastLoginRequired'))
    const u = fetchUrl.trim()
    if (!URL_RE.test(u)) return pushToast(t('upload.toastInvalidUrl'))
    if (full) return pushToast(t('upload.toastQuotaFull'))
    if (bwFull) return pushToast(t('upload.toastBandwidthFull'))
    addUrl(u, opts)
    setFetchUrl('')
    pushToast(t('upload.toastFetchQueued'))
  }

  function copyAllLinks() {
    const urls = items.filter((i) => i.result).map((i) => i.result!.links.url)
    if (!urls.length) return pushToast(t('upload.toastNoLinks'))
    copyText(urls.join('\n'), t('upload.nLinks', { count: urls.length }))
  }

  const albumName = albumId ? (albums.data?.items.find((a) => a.id === albumId)?.name ?? '') : t('upload.noAlbum')
  const summary = `${albumName} · ${visibility === 'public' ? t('upload.public') : t('upload.private')} · ${expiryLabel} · ${maxViewsLabel}`

  return (
    <div
      className={styles.page}
      onDragEnter={(e) => {
        e.preventDefault()
        if (e.dataTransfer.types.includes('Files')) setPageDrag(true)
      }}
      onDragOver={(e) => {
        e.preventDefault()
        if (e.dataTransfer.types.includes('Files')) setPageDrag(true)
      }}
      onDragLeave={(e) => {
        if (e.currentTarget === e.target) setPageDrag(false)
      }}
      onDrop={(e) => {
        e.preventDefault()
        setPageDrag(false)
        setDrag(false)
        acceptFiles(e.dataTransfer.files)
      }}
    >
      {!isGuest && <FirstRunOnboarding show />}

      {pageDrag && <div className={styles.pageDragOverlay}>{t('upload.dropRelease')}</div>}
      <PageHeader kicker="UPLOAD" title={t('upload.title')} extra={<p className={styles.subtitle}>{t('upload.subtitle')}</p>} />

      {isGuest && guestUploadOn && guestLimits && (
        <div className={styles.guestBar}>
          <span className={styles.guestTag}>{t('upload.guestMode')}</span>
          <span>
            {t('upload.guestLimits', {
              size: formatBytes(guestLimits.max_file_size),
              perDay: guestLimits.per_day,
            })}
          </span>
        </div>
      )}

      {/* 窄屏补位：顶栏 cluster 在 ≤900px 隐藏，上传页再露出用量，避免桌面双显 */}
      {!isGuest && quota.data && (
        <div className={styles.quotaRow} data-testid="upload-quota-meters">
          <QuotaBar used={quota.data.used} total={quota.data.total} kind="storage" to="/settings" />
          {bwQuota > 0 && (
            <QuotaBar used={bwUsed} total={bwQuota} kind="bandwidth" to="/settings" />
          )}
        </div>
      )}

      {needLogin && (
        <div className={styles.loginGate} data-testid="login-gate">
          <div className={styles.loginGateTitle}>{t('upload.loginRequiredTitle')}</div>
          <p className={styles.loginGateDesc}>{t('upload.loginRequiredDesc')}</p>
          <Link to={loginTo} className={styles.loginGateCta}>
            {t('upload.loginRequiredCta')}
          </Link>
          {(!!config.data?.help_url?.trim() || !!config.data?.upgrade_url?.trim()) && (
            <div className={styles.loginGateLinks}>
              {!!config.data?.help_url?.trim() && (
                <a href={config.data.help_url.trim()} rel="noopener noreferrer">
                  {t('upload.helpLink')}
                </a>
              )}
              {!!config.data?.help_url?.trim() && !!config.data?.upgrade_url?.trim() && (
                <span className={styles.loginGateDot}>·</span>
              )}
              {!!config.data?.upgrade_url?.trim() && (
                <a href={config.data.upgrade_url.trim()} rel="noopener noreferrer">
                  {t('upload.upgradeLink')}
                </a>
              )}
            </div>
          )}
          <p className={styles.loginGateHint}>{t('upload.loginRequiredHint')}</p>
        </div>
      )}

      <div
        data-testid="dropzone"
        className={[
          styles.dropzone,
          drag && styles.dropzoneDrag,
          needLogin && styles.dropzoneMuted,
        ]
          .filter(Boolean)
          .join(' ')}
        onClick={() => {
          if (needLogin) return pushToast(t('upload.toastLoginRequired'))
          if (full) return pushToast(t('upload.toastQuotaFull'))
          if (bwFull) return pushToast(t('upload.toastBandwidthFull'))
          fileInput.current?.click()
        }}
        onDragOver={(e) => {
          e.preventDefault()
          if (needLogin) return
          if (!drag) setDrag(true)
        }}
        onDragLeave={() => setDrag(false)}
        onDrop={(e) => {
          e.preventDefault()
          setDrag(false)
          acceptFiles(e.dataTransfer.files)
        }}
      >
        {drag && <div className={styles.dragOverlay}>{t('upload.dropRelease')}</div>}
        {(full || bwFull) && (
          <div
            className={styles.fullOverlay}
            onClick={(e) => {
              e.stopPropagation()
              pushToast(full ? t('upload.toastQuotaFull') : t('upload.toastBandwidthFull'))
            }}
          >
            <span className={styles.fullTitle}>
              {full ? t('upload.fullTitle') : t('upload.bandwidthFullTitle')}
            </span>
            <span className={styles.fullDesc}>
              {full ? t('upload.fullDesc') : t('upload.bandwidthFullDesc')}
            </span>
            {!!config.data?.upgrade_url?.trim() && (
              <a
                className={styles.fullSelfHost}
                href={config.data.upgrade_url.trim()}
                rel="noopener noreferrer"
                onClick={(e) => e.stopPropagation()}
              >
                {t('upload.upgradeLink')} →
              </a>
            )}
          </div>
        )}
        <div className={styles.upIcon}>↑</div>
        <div className={styles.dzTitle}>{t('upload.dropTitle')}</div>
        <div className={styles.dzLimit}>
          {needLogin ? (
            t('upload.loginRequiredTitle')
          ) : limits ? (
            `${extLabel(limits.allowedExts)} — MAX ${formatBytes(limits.maxFileSize)}`
          ) : (
            <Skeleton width={220} height={11} />
          )}
        </div>
        <div className={styles.kbdRow}>
          <span className={styles.kbd}>Ctrl</span>
          <span className={styles.kbd}>V</span>
          {t('upload.pasteHint')}
        </div>
        <input
          ref={fileInput}
          type="file"
          accept="image/*"
          multiple
          className={styles.fileInput}
          disabled={needLogin}
          onChange={(e) => {
            if (e.target.files?.length) acceptFiles(e.target.files)
            e.target.value = ''
          }}
        />
      </div>

      <div className={[styles.urlRow, needLogin && styles.urlRowDisabled].filter(Boolean).join(' ')}>
        <span className={styles.urlTag}>URL</span>
        <input
          className={styles.urlInput}
          value={fetchUrl}
          placeholder={t('upload.urlPlaceholder')}
          disabled={needLogin}
          onChange={(e) => setFetchUrl(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') doFetch()
          }}
        />
        <button type="button" className={styles.fetchBtn} disabled={needLogin} onClick={doFetch}>
          {t('upload.fetch')}
        </button>
      </div>

      {!isGuest && (
        <div className={styles.optsBox}>
          <button type="button" className={styles.optsHead} onClick={() => setOptsOpen((v) => !v)}>
            <span className={styles.optsTitle}>
              <span className={styles.optsKicker}>OPTIONS</span>
              {t('upload.options')}
            </span>
            <span className={styles.optsRight}>
              <span className={styles.optsSummary}>{summary}</span>
              <span className={styles.chevron}>{optsOpen ? '▲' : '▼'}</span>
            </span>
          </button>
          {optsOpen && (
            <div className={styles.optsGrid}>
              <div className={styles.optField}>
                <label className={styles.optLabel} htmlFor="opt-album">
                  {t('upload.uploadToAlbum')}
                </label>
                <select
                  id="opt-album"
                  className={styles.optSelect}
                  value={albumId ?? 'none'}
                  onChange={(e) => setAlbumId(e.target.value === 'none' ? null : Number(e.target.value))}
                >
                  <option value="none">{t('upload.noAlbum')}</option>
                  {albums.data?.items.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name}
                    </option>
                  ))}
                </select>
              </div>
              <div className={styles.optField}>
                <span className={styles.optLabel}>{t('upload.visibility')}</span>
                <Segmented<'public' | 'private'>
                  options={[
                    { value: 'public', label: t('upload.public') },
                    { value: 'private', label: t('upload.private') },
                  ]}
                  value={visibility}
                  onChange={setVisibility}
                />
              </div>
              <div className={`${styles.optField} ${styles.optFieldWide}`}>
                <span className={styles.optLabel}>{t('upload.expiry')}</span>
                <Segmented
                  mono
                  options={expiryPresets.map((p) => ({
                    value: p.key,
                    label: expiryPresetLabel(p, t),
                  }))}
                  value={expiryKey}
                  onChange={(k) => {
                    const p = expiryPresets.find((x) => x.key === k)
                    setExpiresIn(p?.sec ?? (expiresCap > 0 ? expiresCap : 0))
                  }}
                />
              </div>
              <div className={`${styles.optField} ${styles.optFieldWide}`}>
                <span className={styles.optLabel}>{t('upload.maxViews')}</span>
                <Segmented
                  mono
                  options={maxViewsPresets.map((p) => ({
                    value: p.key,
                    label: maxViewsPresetLabel(p, t),
                  }))}
                  value={maxViewsKey}
                  onChange={(k) => {
                    const p = maxViewsPresets.find((x) => x.key === k)
                    setMaxViews(p?.n ?? (accessPolicy.max_max_views > 0 ? accessPolicy.max_max_views : 0))
                  }}
                />
              </div>
              {showPolicy && (
                <div className={styles.optField}>
                  <label className={styles.optLabel} htmlFor="opt-policy">
                    {t('upload.storagePolicy')}
                  </label>
                  <select
                    id="opt-policy"
                    className={styles.optSelect}
                    value={policyId ?? 'default'}
                    onChange={(e) => setPolicyId(e.target.value === 'default' ? null : Number(e.target.value))}
                  >
                    <option value="default">{t('upload.followDefault')}</option>
                    {policies.data?.map((po) => (
                      <option key={po.id} value={po.id}>
                        {po.name}
                      </option>
                    ))}
                  </select>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {items.length > 0 && (
        <>
          <div className={styles.queueHead}>
            <span className={styles.queueKicker}>QUEUE — {items.length}</span>
            <span className={styles.queueActions}>
              <button type="button" className={styles.textBtn} onClick={copyAllLinks}>
                {t('upload.copyAllLinks')}
              </button>
              <button type="button" className={styles.textBtn} onClick={clearDone}>
                {t('upload.clearDone')}
              </button>
            </span>
          </div>
          <div className={styles.queueList}>
            {items.map((i) => (
              <UploadCard key={i.id} item={i} />
            ))}
          </div>
        </>
      )}
    </div>
  )
}
