import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { defaultFilter, useAlbums, useDeleteImage, useImages, useUpdateAlbum, useUpdateImage } from '../../api/hooks'
import type { ImageItem } from '../../api/types'
import { useT } from '../../i18n'
import { formatDate } from '../../lib/format'
import { useGlobal } from '../../store'
import { Button } from '../../ui/Button'
import { EmptyState } from '../../ui/EmptyState'
import { BatchBar } from '../images/BatchBar'
import { DetailModal } from '../images/DetailModal'
import { ImageGrid } from '../images/ImageGrid'
import styles from './AlbumDetailPage.module.css'

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
      <div className={styles.page}>
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

  const quickVis = (item: ImageItem) =>
    update.mutate({ key: item.key, body: { visibility: item.visibility === 'public' ? 'private' : 'public' } })

  return (
    <div className={styles.page}>
      <div className={styles.head}>
        <div className={styles.headLeft}>
          <Link to="/albums" className={styles.back}>
            ← ALBUMS
          </Link>
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
              <button type="button" className={styles.renameSave} onClick={commitRename}>{t('albums.save')}</button>
            </div>
          ) : (
            <div className={styles.titleRow}>
              <h1 className={styles.title}>{album?.name ?? '…'}</h1>
              <button
                type="button"
                className={styles.renameLink}
                onClick={() => {
                  setRenameVal(album?.name ?? '')
                  setRenaming(true)
                }}
              >
                {t('albums.rename')}
              </button>
              {album && <span className={styles.privacyTag}>{album.visibility === 'public' ? 'PUBLIC' : 'PRIVATE'}</span>}
            </div>
          )}
        </div>
        <div className={styles.headRight}>
          <Button onClick={togglePrivacy} disabled={updateAlbum.isPending}>
            {album?.visibility === 'public' ? t('albums.setPrivate') : t('albums.setPublic')}
          </Button>
          <Link to={album ? `/?album=${album.id}` : '/'}>
            <Button variant="primary">{t('albums.uploadToAlbum')}</Button>
          </Link>
        </div>
      </div>
      <div className={styles.metaLine}>
        {album ? t('albums.detailMeta', { count: album.image_count, date: formatDate(album.created_at) }) : ''}
      </div>

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
