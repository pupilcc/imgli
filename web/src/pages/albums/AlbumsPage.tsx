import { useState } from 'react'
import { useNavigate } from 'react-router'
import { useAlbums, useCreateAlbum, useDeleteAlbum } from '../../api/hooks'
import type { Album } from '../../api/types'
import { useT } from '../../i18n'
import { formatDate } from '../../lib/format'
import { useGlobal } from '../../store'
import { Button } from '../../ui/Button'
import { EmptyState } from '../../ui/EmptyState'
import { Modal } from '../../ui/Modal'
import { PageHeader } from '../../shell/PageHeader'
import styles from './AlbumsPage.module.css'

export function AlbumsPage() {
  const { t } = useT()
  const albums = useAlbums()
  const create = useCreateAlbum()
  const remove = useDeleteAlbum()
  const pushToast = useGlobal((s) => s.pushToast)
  const navigate = useNavigate()
  const [showNew, setShowNew] = useState(false)
  const [newName, setNewName] = useState('')
  const [newVis, setNewVis] = useState<'public' | 'private'>('public')
  const [delTarget, setDelTarget] = useState<Album | null>(null)

  const items = albums.data?.items ?? []

  function createAlbum() {
    const name = newName.trim()
    if (!name) return pushToast(t('albums.nameRequired'))
    create.mutate(
      { name, visibility: newVis },
      {
        onSuccess: () => {
          setShowNew(false)
          setNewName('')
          pushToast(t('albums.created'))
        },
      },
    )
  }

  function doDelete(withImages: boolean) {
    if (!delTarget) return
    remove.mutate(
      { id: delTarget.id, withImages },
      {
        onSuccess: () => {
          pushToast(withImages ? t('albums.deletedWithImages') : t('albums.deletedKeepImages'))
          setDelTarget(null)
        },
      },
    )
  }

  return (
    <div className={styles.page}>
      <PageHeader
        kicker="ALBUMS"
        title={t('albums.title')}
        extra={
          <Button variant="primary" onClick={() => setShowNew(true)}>
            {t('albums.newAlbum')}
          </Button>
        }
      />

      {albums.isLoading ? null : items.length === 0 ? (
        <EmptyState title={t('albums.emptyTitle')} desc={t('albums.emptyDesc')}>
          <Button variant="primary" onClick={() => setShowNew(true)}>
            {t('albums.newFirstAlbum')}
          </Button>
        </EmptyState>
      ) : (
        <div className={styles.grid}>
          {items.map((a) => (
            <div key={a.id} className={styles.card} onClick={() => navigate(`/albums/${a.id}`)}>
              <div className={styles.collage}>
                <div className={styles.coverMain}>
                  {a.cover_key ? (
                    <img className={styles.coverImg} src={`/t/${a.cover_key}.jpg`} alt="" loading="lazy" />
                  ) : (
                    <span className={styles.coverPh}>COVER</span>
                  )}
                </div>
                <div className={`stripe ${styles.coverCell}`} />
                <div className={`stripe ${styles.coverCell}`}>
                  {a.image_count > 1 && <span className={styles.moreLabel}>+{a.image_count - 1}</span>}
                </div>
              </div>
              <span className={[styles.privacy, a.visibility === 'public' && styles.privacyPub].filter(Boolean).join(' ')}>
                {a.visibility === 'public' ? 'PUBLIC' : 'PRIVATE'}
              </span>
              <div className={styles.meta}>
                <div className={styles.metaLeft}>
                  <div className={styles.name}>{a.name}</div>
                  <div className={styles.sub}>
                    {t('albums.meta', { count: a.image_count, date: formatDate(a.created_at) })}
                  </div>
                </div>
                <button
                  type="button"
                  title={t('albums.deleteAlbumTitle')}
                  className={styles.delBtn}
                  onClick={(e) => {
                    e.stopPropagation()
                    setDelTarget(a)
                  }}
                >
                  ×
                </button>
              </div>
            </div>
          ))}
          <div className={styles.ghost} onClick={() => setShowNew(true)}>
            <div className={styles.ghostIcon}>＋</div>
            <span className={styles.ghostText}>{t('albums.newAlbumShort')}</span>
          </div>
        </div>
      )}

      <Modal open={showNew} onClose={() => setShowNew(false)} width={400}>
        <div className={styles.modalKicker}>NEW ALBUM</div>
        <div className={styles.modalTitle}>{t('albums.newAlbumTitle')}</div>
        <div className={styles.field}>
          <label className={styles.label} htmlFor="album-name">{t('albums.nameLabel')}</label>
          <input
            id="album-name"
            className={styles.input}
            placeholder={t('albums.namePlaceholder')}
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
          />
        </div>
        <div className={styles.field}>
          <span className={styles.label}>{t('albums.visibility')}</span>
          <div className={styles.visRow}>
            <button
              type="button"
              className={[styles.visBtn, newVis === 'public' && styles.visActive].filter(Boolean).join(' ')}
              onClick={() => setNewVis('public')}
            >
              {t('albums.visPublic')}
            </button>
            <button
              type="button"
              className={[styles.visBtn, styles.visBl, newVis === 'private' && styles.visActive].filter(Boolean).join(' ')}
              onClick={() => setNewVis('private')}
            >
              {t('albums.visPrivate')}
            </button>
          </div>
        </div>
        <Button variant="primary" className={styles.createBtn} onClick={createAlbum} disabled={create.isPending}>
          {t('albums.create')}
        </Button>
      </Modal>

      <Modal open={!!delTarget} onClose={() => setDelTarget(null)}>
        <div className={styles.modalKickerErr}>DELETE ALBUM</div>
        <div className={styles.modalTitle}>{t('albums.deleteConfirm', { name: delTarget?.name ?? '' })}</div>
        <p className={styles.modalDesc}>
          {t('albums.deleteDesc', { count: delTarget?.image_count ?? 0 })}
        </p>
        <div className={styles.delCol}>
          <button type="button" className={styles.delOpt} onClick={() => doDelete(false)} disabled={remove.isPending}>
            {t('albums.deleteAlbumOnly')}
            <span className={styles.delOptSub}>{t('albums.deleteAlbumOnlySub')}</span>
          </button>
          <button type="button" className={`${styles.delOpt} ${styles.delOptErr}`} onClick={() => doDelete(true)} disabled={remove.isPending}>
            {t('albums.deleteWithImages')}
            <span className={styles.delOptSub}>{t('albums.deleteWithImagesSub', { count: delTarget?.image_count ?? 0 })}</span>
          </button>
          <button type="button" className={styles.delCancel} onClick={() => setDelTarget(null)}>
            {t('albums.cancel')}
          </button>
        </div>
      </Modal>
    </div>
  )
}
