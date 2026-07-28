import { useEffect, useState } from 'react'
import { useAlbums, useBatchImages } from '../../api/hooks'
import type { BatchResult, ImageItem } from '../../api/types'
import { useT } from '../../i18n'
import { copyText } from '../../lib/copy'
import { useGlobal } from '../../store'
import { Button } from '../../ui/Button'
import { Modal } from '../../ui/Modal'
import styles from './BatchBar.module.css'

interface Props {
  selected: Set<string>
  items: ImageItem[]
  onClear(): void
}

function summarize(
  results: BatchResult[],
  verb: string,
  t: (key: string, vars?: Record<string, string | number>) => string,
): string {
  const ok = results.filter((r) => r.ok).length
  const bad = results.length - ok
  return bad === 0
    ? t('images.batchDone', { verb, ok })
    : t('images.batchDonePartial', { verb, ok, bad })
}

export function BatchBar({ selected, items, onClear }: Props) {
  const { t } = useT()
  const batch = useBatchImages()
  const albums = useAlbums()
  const pushToast = useGlobal((s) => s.pushToast)
  const [modal, setModal] = useState<'vis' | 'move' | null>(null)
  const [moveTo, setMoveTo] = useState<number | 'none'>('none')
  const [delArmed, setDelArmed] = useState(false)
  useEffect(() => {
    if (!delArmed) return
    const timer = setTimeout(() => setDelArmed(false), 2500)
    return () => clearTimeout(timer)
  }, [delArmed])

  if (selected.size === 0) return null
  const keys = [...selected]

  const CHUNK = 100

  const run = (body: { action: 'delete' | 'visibility' | 'move'; keys: string[]; visibility?: string; album_id?: number | null }, verb: string) => {
    const chunks: string[][] = []
    for (let i = 0; i < body.keys.length; i += CHUNK) chunks.push(body.keys.slice(i, i + CHUNK))
    ;(async () => {
      const all: BatchResult[] = []
      try {
        for (const chunkKeys of chunks) {
          const d = await batch.mutateAsync({ ...body, keys: chunkKeys })
          all.push(...d.results)
        }
        pushToast(summarize(all, verb, t))
        setModal(null)
        onClear()
      } catch {
        pushToast(all.length ? t('images.batchRestFailed', { summary: summarize(all, verb, t) }) : t('images.opFailed'))
      }
    })()
  }

  const copyAll = () => {
    const urls = items.filter((i) => selected.has(i.key)).map((i) => i.links.url)
    copyText(urls.join('\n'), t('images.nLinks', { count: urls.length }))
  }

  const openModal = (m: 'vis' | 'move') => {
    setDelArmed(false)
    if (m === 'move') setMoveTo('none')
    setModal(m)
  }

  return (
    <>
      <div className={styles.bar}>
        <span className={styles.count}>{t('images.selected', { count: selected.size })}</span>
        <button type="button" className={styles.act} onClick={copyAll}>{t('images.copyLinks')}</button>
        <button type="button" className={styles.act} onClick={() => openModal('vis')}>{t('images.changeVisibility')}</button>
        <button type="button" className={styles.act} onClick={() => openModal('move')}>{t('images.moveToAlbum')}</button>
        <button
          type="button"
          className={[styles.act, delArmed && styles.actDanger].filter(Boolean).join(' ')}
          onClick={() => {
            if (delArmed) {
              setDelArmed(false)
              run({ action: 'delete', keys }, t('images.verbDelete'))
            } else setDelArmed(true)
          }}
        >
          {delArmed ? t('images.confirmDeleteQ') : t('images.delete')}
        </button>
        <button type="button" className={styles.close} title={t('images.cancelSelect')} onClick={onClear}>×</button>
      </div>

      <Modal open={modal === 'vis'} onClose={() => setModal(null)}>
        <div className={styles.modalKicker}>BATCH VISIBILITY</div>
        <div className={styles.modalTitle}>{t('images.setAs', { count: selected.size })}</div>
        <div className={styles.modalCol}>
          <Button onClick={() => run({ action: 'visibility', keys, visibility: 'public' }, t('images.verbPublic'))}>
            {t('images.setPublic')}
          </Button>
          <Button onClick={() => run({ action: 'visibility', keys, visibility: 'private' }, t('images.verbPrivate'))}>
            {t('images.setPrivate')}
          </Button>
        </div>
      </Modal>

      <Modal open={modal === 'move'} onClose={() => setModal(null)}>
        <div className={styles.modalKicker}>BATCH MOVE</div>
        <div className={styles.modalTitle}>{t('images.moveN', { count: selected.size })}</div>
        <select
          className={styles.modalSelect}
          aria-label={t('images.targetAlbum')}
          value={String(moveTo)}
          onChange={(e) => setMoveTo(e.target.value === 'none' ? 'none' : Number(e.target.value))}
        >
          <option value="none">{t('images.removeFromAlbum')}</option>
          {albums.data?.items.map((a) => (
            <option key={a.id} value={a.id}>{a.name}</option>
          ))}
        </select>
        <div className={styles.modalCol}>
          <Button
            variant="primary"
            onClick={() => run({ action: 'move', keys, album_id: moveTo === 'none' ? null : moveTo }, t('images.verbMove'))}
          >
            {t('images.confirmMove')}
          </Button>
        </div>
      </Modal>
    </>
  )
}
