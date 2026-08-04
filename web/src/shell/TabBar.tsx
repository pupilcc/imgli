import { NavLink } from 'react-router'
import { useT } from '../i18n'
import { cn } from '../lib/cn'

const TABS = [
  { to: '/', glyph: '↑', labelKey: 'nav.tabUpload', end: true },
  { to: '/images', glyph: '⊞', labelKey: 'nav.tabImages', end: false },
  { to: '/albums', glyph: '▤', labelKey: 'nav.tabAlbums', end: false },
  { to: '/settings', glyph: '○', labelKey: 'nav.tabMine', end: false },
]

/** 移动端底部 4-Tab（<768px 显示）。 */
export function TabBar() {
  const { t } = useT()
  return (
    <nav
      className="fixed inset-x-0 bottom-0 z-20 hidden h-14 border-t border-border bg-surface max-md:flex"
      data-testid="tabbar"
    >
      {TABS.map((tab) => (
        <NavLink
          key={tab.to}
          to={tab.to}
          end={tab.end}
          className={({ isActive }) =>
            cn(
              'flex min-h-11 flex-1 flex-col items-center justify-center gap-0.5 text-muted hover:text-ink',
              isActive && 'text-ink shadow-[inset_0_2px_0_var(--text)]',
            )
          }
        >
          <span className="text-[15px] leading-none">{tab.glyph}</span>
          <span className="text-[11px] font-semibold">{t(tab.labelKey)}</span>
        </NavLink>
      ))}
    </nav>
  )
}
