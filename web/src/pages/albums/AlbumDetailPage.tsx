import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { defaultFilter, useAlbums, useDeleteImage, useImages, useUpdateAlbum, useUpdateImage } from '../../api/hooks'
import type { ImageItem } from '../../api/types'
import { useT } from '../../i18n'
import { copyText } from '../../lib/copy'
import { formatDate } from '../../lib/format'
import { useGlobal } from '../../store'
import { Button } from '../../ui/Button'
import { EmptyState } from '../../ui/EmptyState'
import { Segmented } from '../../ui/Segmented'
import { BatchBar } from '../images/BatchBar'
import { DetailModal } from '../images/DetailModal'
import { ImageGrid } from '../images/ImageGrid'
import type { AlbumPublicMode } from './albumPublicView'

export function AlbumDetailPage() {
  const { t } = useT()
  const { id: idParam } = useParams()
  const id = Number(idParam)
  const albums = useAlbums()
  const album = albums.data?.items.find((a) => a.id === id)
  const images = useImages(useMemo(() => ({ ...defaultFilter, album: id }), [id]))
  const update = useUpdateImage()
  const removeImg = useDeleteImage()
  const updateAlbum = useUpdateAlbum()
  const pushToast = useGlobal((s) => s.pushToast)
  const navigate = useNavigate()
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [focusKey, setFocusKey] = useState<string | null>(null)
  const [renaming, setRenaming] = useState(false)
  const [renameVal, setRenameVal] = useState('')
  const sentinelRef = useRef<HTMLDivElement>(null)

  const items = useMemo(() => images.data?.pages.flatMap((p) => p.items) ?? [], [images.data])

  useEffect(() => {
    setSelected((s) => {
      const live = new Set(items.map((i) => i.key))
      const next = new Set([...s].filter((k) => live.has(k)))
      return next.size === s.size ? s : next
    })
  }, [items])

  const { hasNextPage, isFetchingNextPage, fetchNextPage } = images
  useEffect(() => {
    const el = sentinelRef.current
    if (!el) return
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting) && hasNextPage && !isFetchingNextPage) fetchNextPage()
    })
    io.observe(el)
    return () => io.disconnect()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage, items.length])

  if (!albums.isLoading && !album) {
    return (
      <div className="mx-auto max-w-[1120px] pt-11">
        <EmptyState badge="404" title={t('albums.notFound')} desc={t('albums.notFoundDesc')}>
          <Button variant="primary" onClick={() => navigate('/albums')}>
            {t('albums.backToAlbums')}
          </Button>
        </EmptyState>
      </div>
    )
  }

  const commitRename = () => {
    const name = renameVal.trim()
    if (!name || !album || name === album.name) return setRenaming(false)
    updateAlbum.mutate(
      { id, body: { name } },
      {
        onSuccess: () => {
          setRenaming(false)
          pushToast(t('albums.renamed'))
        },
      },
    )
  }

  const togglePrivacy = () => {
    if (!album) return
    updateAlbum.mutate({ id, body: { visibility: album.visibility === 'public' ? 'private' : 'public' } })
  }

  const setDefaultView = (v: AlbumPublicMode) => {
    if (!album || album.default_view === v) return
    updateAlbum.mutate(
      { id, body: { default_view: v } },
      { onSuccess: () => pushToast(t('albums.defaultViewSaved')) },
    )
  }

  const quickVis = (item: ImageItem) =>
    update.mutate({ key: item.key, body: { visibility: item.visibility === 'public' ? 'private' : 'public' } })

  const defaultView: AlbumPublicMode = album?.default_view === 'immersive' ? 'immersive' : 'gallery'

  return (
    <div className="mx-auto max-w-[1120px] pt-11">
      <div className="mb-6 flex animate-[fadeIn_0.2s] items-end justify-between border-b border-border pb-[18px]">
        <div className="min-w-0">
          <Link
            to="/albums"
            className="mb-2 inline-block font-mono text-[11px] tracking-[0.14em] text-muted hover:text-ink"
          >
            ← ALBUMS
          </Link>
          {renaming ? (
            <div className="flex gap-1.5">
              <input
                className="w-[220px] rounded-sm border border-muted bg-bg px-2.5 py-1 font-inherit text-xl font-bold text-ink outline-none"
                autoFocus
                value={renameVal}
                onChange={(e) => setRenameVal(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') commitRename()
                }}
              />
              <button
                type="button"
                className="cursor-pointer rounded-sm border-0 bg-btn px-3.5 py-[7px] text-xs font-bold text-btn-text"
                onClick={commitRename}
              >
                {t('albums.save')}
              </button>
            </div>
          ) : (
            <div className="flex items-baseline gap-3">
              <h1 className="m-0 text-[26px] font-bold tracking-[-0.015em]">{album?.name ?? '…'}</h1>
              <button
                type="button"
                className="cursor-pointer border-0 bg-transparent p-0 text-[11.5px] text-muted underline hover:text-ink"
                onClick={() => {
                  setRenameVal(album?.name ?? '')
                  setRenaming(true)
                }}
              >
                {t('albums.rename')}
              </button>
              {album && (
                <span className="rounded-[2px] border border-border px-[7px] py-0.5 font-mono text-[9.5px] tracking-[0.1em] text-muted">
                  {album.visibility === 'public' ? 'PUBLIC' : 'PRIVATE'}
                </span>
              )}
            </div>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          {album?.visibility === 'public' && (
            <>
              <Button
                onClick={() => {
                  const url = `${window.location.origin}/a/${album.id}`
                  void copyText(url, t('albums.publicLink'))
                }}
              >
                {t('albums.copyPublicLink')}
              </Button>
              <Button
                onClick={() => {
                  window.open(`/a/${album.id}`, '_blank', 'noopener,noreferrer')
                }}
              >
                {t('albums.openPublicLink')}
              </Button>
            </>
          )}
          <Button onClick={togglePrivacy} disabled={updateAlbum.isPending}>
            {album?.visibility === 'public' ? t('albums.setPrivate') : t('albums.setPublic')}
          </Button>
          <Link to={album ? `/?album=${album.id}` : '/'}>
            <Button variant="primary">{t('albums.uploadToAlbum')}</Button>
          </Link>
        </div>
      </div>
      <div className="mb-3.5 font-mono text-xs-plus tracking-[0.06em] text-muted">
        {album ? t('albums.detailMeta', { count: album.image_count, date: formatDate(album.created_at) }) : ''}
      </div>
      {album?.visibility === 'public' && (
        <div
          className="mb-5 flex flex-wrap items-center justify-between gap-3 rounded-sm border border-border bg-surface px-3.5 py-3"
          data-testid="album-default-view"
        >
          <div className="min-w-0">
            <div className="text-[12.5px] font-semibold text-ink">{t('albums.defaultViewLabel')}</div>
            <div className="mt-0.5 text-[11.5px] text-muted">{t('albums.defaultViewHint')}</div>
          </div>
          <Segmented<AlbumPublicMode>
            compact
            options={[
              { value: 'gallery', label: t('albums.modeGallery') },
              { value: 'immersive', label: t('albums.modeImmersive') },
            ]}
            value={defaultView}
            onChange={setDefaultView}
          />
        </div>
      )}

      {items.length === 0 && !images.isLoading ? (
        <EmptyState title={t('albums.detailEmptyTitle')} desc={t('albums.detailEmptyDesc')}>
          <Link to={album ? `/?album=${album.id}` : '/'}>
            <Button variant="primary">{t('albums.goUpload')}</Button>
          </Link>
        </EmptyState>
      ) : (
        <ImageGrid
          items={items}
          view="grid"
          selected={selected}
          sentinelRef={sentinelRef}
          loadingMore={isFetchingNextPage}
          onToggleSelect={(k) =>
            setSelected((s) => {
              const n = new Set(s)
              if (n.has(k)) n.delete(k)
              else n.add(k)
              return n
            })
          }
          onOpen={setFocusKey}
          onQuickVis={quickVis}
          onQuickDel={(k) => removeImg.mutate(k)}
        />
      )}

      <BatchBar selected={selected} items={items} onClear={() => setSelected(new Set())} />
      {focusKey && (
        <DetailModal items={items} focusKey={focusKey} onClose={() => setFocusKey(null)} onNavigate={setFocusKey} />
      )}
    </div>
  )
}
