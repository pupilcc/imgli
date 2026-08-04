import { useAlbums, type ImagesFilter } from '../../api/hooks'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { useGlobal, type View } from '../../store'

const FORMATS = ['ALL', 'PNG', 'JPG', 'GIF', 'WEBP'] as const

interface Props {
  filter: ImagesFilter
  onFilter(f: ImagesFilter): void
  allSelected: boolean
  onToggleAll(): void
  selectable: boolean
}

const selectCls =
  'h-[34px] cursor-pointer rounded-sm border border-border bg-surface px-2.5 font-inherit text-xs font-semibold text-ink outline-none'

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
    <div className="mb-5 flex flex-wrap items-center gap-2.5">
      <div className="flex h-[34px] w-full items-center gap-2 rounded-sm border border-border bg-surface px-3 md:w-[210px]">
        <span className="text-[13px] text-muted" aria-hidden>
          ⌕
        </span>
        <input
          className="w-full border-0 bg-transparent font-inherit text-[13px] text-ink outline-none"
          placeholder={t('images.searchPlaceholder')}
          value={filter.q}
          onChange={(e) => onFilter({ ...filter, q: e.target.value })}
        />
      </div>
      <div className="flex overflow-hidden overflow-x-auto rounded-sm border border-border">
        {FORMATS.map((fmt, i) => {
          const active = filter.format === fmt
          return (
            <button
              key={fmt}
              type="button"
              className={cn(
                'h-8 cursor-pointer border-0 px-3 font-mono text-[11px] font-semibold tracking-[0.05em]',
                i > 0 && 'border-l border-border',
                active ? 'bg-btn text-btn-text' : 'bg-surface text-muted hover:bg-soft',
              )}
              onClick={() => onFilter({ ...filter, format: fmt })}
            >
              {fmt}
            </button>
          )
        })}
      </div>
      <select
        className={selectCls}
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
        className={selectCls}
        aria-label={t('images.visibilityFilter')}
        value={filter.visibility}
        onChange={(e) => onFilter({ ...filter, visibility: e.target.value as ImagesFilter['visibility'] })}
      >
        <option value="all">{t('images.allVisibility')}</option>
        <option value="public">{t('images.public')}</option>
        <option value="private">{t('images.private')}</option>
      </select>
      <select
        className={selectCls}
        aria-label={t('images.sort')}
        value={filter.sort}
        onChange={(e) => onFilter({ ...filter, sort: e.target.value as ImagesFilter['sort'] })}
      >
        <option value="date">{t('images.sortDate')}</option>
        <option value="size">{t('images.sortSize')}</option>
        <option value="name">{t('images.sortName')}</option>
      </select>
      <div className="ml-0 flex items-center gap-2.5 md:ml-auto">
        {selectable && (
          <button
            type="button"
            className="h-[34px] cursor-pointer rounded-sm border border-border bg-surface px-3 text-xs font-semibold text-muted hover:bg-soft hover:text-ink"
            onClick={onToggleAll}
          >
            {allSelected ? t('images.deselectAll') : t('images.selectAll')}
          </button>
        )}
        <div className="flex overflow-hidden rounded-sm border border-border">
          {views.map((o, i) => {
            const active = view === o.v
            return (
              <button
                key={o.v}
                type="button"
                title={o.title}
                className={cn(
                  'flex h-8 cursor-pointer items-center gap-1.5 border-0 px-[11px] text-xs font-semibold transition-[background,color] duration-150',
                  i > 0 && 'border-l border-border',
                  active ? 'bg-btn text-btn-text' : 'bg-surface text-muted hover:bg-soft',
                )}
                onClick={() => setView(o.v)}
              >
                <span className="text-[13px] leading-none">{o.glyph}</span>
                {o.label}
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
