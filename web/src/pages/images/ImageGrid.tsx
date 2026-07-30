import type { RefObject } from 'react'
import type { ImageItem } from '../../api/types'
import { useT } from '../../i18n'
import { copyText } from '../../lib/copy'
import { formatBytes, formatDate } from '../../lib/format'
import type { View } from '../../store'
import { ArmedButton } from '../../ui/ArmedButton'
import styles from './ImageGrid.module.css'

interface CardActions {
  onToggleSelect(key: string): void
  onOpen(key: string): void
  onQuickVis(item: ImageItem): void
  onQuickDel(key: string): void
}

interface Props extends CardActions {
  items: ImageItem[]
  view: View
  selected: Set<string>
  sentinelRef: RefObject<HTMLDivElement | null>
  loadingMore: boolean
}

/** 卡内删除钮：第一击转红「确认删除」，2.5s 未确认还原。 */
function QuickDel({ onConfirm }: { onConfirm(): void }) {
  const { t } = useT()
  return (
    <ArmedButton
      title={t('images.delete')}
      armedTitle={t('images.confirmDelete')}
      className={[styles.quickBtn, styles.quickDel].join(' ')}
      armedClassName={styles.quickDelArmed}
      onConfirm={onConfirm}
    >
      ×
    </ArmedButton>
  )
}

function Check({ item, selected, onToggleSelect }: { item: ImageItem; selected: boolean } & Pick<CardActions, 'onToggleSelect'>) {
  const { t } = useT()
  return (
    <button
      type="button"
      title={t('images.select')}
      className={[styles.check, selected && styles.checkOn].filter(Boolean).join(' ')}
      onClick={(e) => {
        e.stopPropagation()
        onToggleSelect(item.key)
      }}
    >
      {selected ? '✓' : ''}
    </button>
  )
}

function HoverActions({ item, onQuickVis, onQuickDel }: { item: ImageItem } & Pick<CardActions, 'onQuickVis' | 'onQuickDel'>) {
  const { t } = useT()
  return (
    <div className={styles.hoverBar}>
      <button
        type="button"
        title={t('images.copyUrlTitle')}
        className={styles.quickBtn}
        onClick={(e) => {
          e.stopPropagation()
          copyText(item.links.url, t('images.linkNameUrl'))
        }}
      >
        ⧉
      </button>
      <button
        type="button"
        title={t('images.toggleVisibility')}
        className={styles.quickBtn}
        onClick={(e) => {
          e.stopPropagation()
          onQuickVis(item)
        }}
      >
        {item.visibility === 'private' ? '◌' : '◉'}
      </button>
      <QuickDel onConfirm={() => onQuickDel(item.key)} />
    </div>
  )
}

function Card({ item, view, selected, ...a }: { item: ImageItem; view: View; selected: boolean } & CardActions) {
  const { t } = useT()
  return (
    <div
      className={[styles.card, view === 'masonry' && styles.cardMasonry, selected && styles.cardSelected]
        .filter(Boolean)
        .join(' ')}
      onClick={() => a.onOpen(item.key)}
    >
      <div
        className={styles.thumbBox}
        style={view === 'masonry' ? { aspectRatio: `${item.width || 1} / ${item.height || 1}` } : undefined}
      >
        <img className={styles.thumb} src={item.links.thumbnail_url} alt={item.name} loading="lazy" />
        <Check item={item} selected={selected} onToggleSelect={a.onToggleSelect} />
        {item.visibility === 'private' && <span className={styles.privBadge}>{t('images.privateBadge')}</span>}
        <HoverActions item={item} onQuickVis={a.onQuickVis} onQuickDel={a.onQuickDel} />
      </div>
      <div className={styles.meta}>
        <span className={styles.metaName}>{item.name}</span>
        <span className={styles.metaSize}>{formatBytes(item.size)}</span>
      </div>
    </div>
  )
}

function ListRow({ item, selected, ...a }: { item: ImageItem; selected: boolean } & CardActions) {
  const { t } = useT()
  return (
    <div className={styles.row} onClick={() => a.onOpen(item.key)}>
      <Check item={item} selected={selected} onToggleSelect={a.onToggleSelect} />
      <img className={styles.rowThumb} src={item.links.thumbnail_url} alt="" loading="lazy" />
      <span className={styles.rowName}>{item.name}</span>
      <span className={styles.rowMuted}>{item.width} × {item.height}</span>
      <span className={styles.rowMuted}>{formatBytes(item.size)}</span>
      <span className={styles.rowMuted}>{item.ext.toUpperCase()}</span>
      <span className={[styles.visTag, item.visibility === 'public' && styles.visPub].filter(Boolean).join(' ')}>
        {item.visibility === 'public' ? 'PUBLIC' : 'PRIVATE'}
      </span>
      <span className={styles.rowMuted}>{formatDate(item.created_at)}</span>
      <div className={styles.rowActions}>
        <button
          type="button"
          title={t('images.copyUrlTitle')}
          className={styles.quickBtn}
          onClick={(e) => {
            e.stopPropagation()
            copyText(item.links.url, t('images.linkNameUrl'))
          }}
        >
          ⧉
        </button>
        <QuickDel onConfirm={() => a.onQuickDel(item.key)} />
      </div>
    </div>
  )
}

export function ImageGrid({ items, view, selected, sentinelRef, loadingMore, ...a }: Props) {
  const { t } = useT()
  return (
    <>
      {view === 'list' ? (
        <div className={styles.listBox}>
          <div className={styles.listHead}>
            <span></span><span></span><span>{t('images.colName')}</span><span>{t('images.colDims')}</span><span>{t('images.colSize')}</span>
            <span>{t('images.colFormat')}</span><span>{t('images.colVisibility')}</span><span>{t('images.colUploaded')}</span><span></span>
          </div>
          {items.map((i) => (
            <ListRow key={i.key} item={i} selected={selected.has(i.key)} {...a} />
          ))}
        </div>
      ) : (
        <div className={view === 'masonry' ? styles.masonry : styles.grid}>
          {items.map((i) => (
            <Card key={i.key} item={i} view={view} selected={selected.has(i.key)} {...a} />
          ))}
        </div>
      )}
      <div ref={sentinelRef} className={styles.sentinel}>
        {loadingMore && <span className={styles.loadingMore}>{t('images.loadingMore')}</span>}
      </div>
    </>
  )
}
