import { useAlbums, type ImagesFilter } from '../../api/hooks'
import { useT } from '../../i18n'
import { useGlobal, type View } from '../../store'
import styles from './Toolbar.module.css'

const FORMATS = ['ALL', 'PNG', 'JPG', 'GIF', 'WEBP'] as const

interface Props {
  filter: ImagesFilter
  onFilter(f: ImagesFilter): void
  allSelected: boolean
  onToggleAll(): void
  selectable: boolean
}

export function Toolbar({ filter, onFilter, allSelected, onToggleAll, selectable }: Props) {
  const { t } = useT()
  const view = useGlobal((s) => s.view)
  const setView = useGlobal((s) => s.setView)
  const albums = useAlbums()
  const views: { v: View; glyph: string; label: string; title: string }[] = [
    { v: 'masonry', glyph: '≣', label: t('images.viewMasonry'), title: t('images.viewMasonryTitle') },
    { v: 'grid', glyph: '⊞', label: t('images.viewGrid'), title: t('images.viewGridTitle') },
    { v: 'list', glyph: '☰', label: t('images.viewList'), title: t('images.viewListTitle') },
  ]
  return (
    <div className={styles.bar}>
      <div className={styles.search}>
        <span className={styles.searchIcon}>⌕</span>
        <input
          className={styles.searchInput}
          placeholder={t('images.searchPlaceholder')}
          value={filter.q}
          onChange={(e) => onFilter({ ...filter, q: e.target.value })}
        />
      </div>
      <div className={styles.chips}>
        {FORMATS.map((fmt, i) => (
          <button
            key={fmt}
            type="button"
            className={[styles.chip, i > 0 && styles.chipBl, filter.format === fmt && styles.chipActive]
              .filter(Boolean)
              .join(' ')}
            onClick={() => onFilter({ ...filter, format: fmt })}
          >
            {fmt}
          </button>
        ))}
      </div>
      <select
        className={styles.select}
        aria-label={t('images.albumFilter')}
        value={String(filter.album)}
        onChange={(e) => {
          const v = e.target.value
          onFilter({ ...filter, album: v === 'all' || v === 'none' ? v : Number(v) })
        }}
      >
        <option value="all">{t('images.allAlbums')}</option>
        {albums.data?.items.map((a) => (
          <option key={a.id} value={a.id}>
            {a.name}
          </option>
        ))}
        <option value="none">{t('images.uncategorized')}</option>
      </select>
      <select
        className={styles.select}
        aria-label={t('images.visibilityFilter')}
        value={filter.visibility}
        onChange={(e) => onFilter({ ...filter, visibility: e.target.value as ImagesFilter['visibility'] })}
      >
        <option value="all">{t('images.allVisibility')}</option>
        <option value="public">{t('images.public')}</option>
        <option value="private">{t('images.private')}</option>
      </select>
      <select
        className={styles.select}
        aria-label={t('images.sort')}
        value={filter.sort}
        onChange={(e) => onFilter({ ...filter, sort: e.target.value as ImagesFilter['sort'] })}
      >
        <option value="date">{t('images.sortDate')}</option>
        <option value="size">{t('images.sortSize')}</option>
        <option value="name">{t('images.sortName')}</option>
      </select>
      <div className={styles.right}>
        {selectable && (
          <button type="button" className={styles.selectAll} onClick={onToggleAll}>
            {allSelected ? t('images.deselectAll') : t('images.selectAll')}
          </button>
        )}
        <div className={styles.views}>
          {views.map((o, i) => (
            <button
              key={o.v}
              type="button"
              title={o.title}
              className={[styles.viewBtn, i > 0 && styles.chipBl, view === o.v && styles.viewActive]
                .filter(Boolean)
                .join(' ')}
              onClick={() => setView(o.v)}
            >
              <span className={styles.viewGlyph}>{o.glyph}</span>
              {o.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
