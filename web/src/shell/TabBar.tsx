import { NavLink } from 'react-router'
import { useT } from '../i18n'
import styles from './TabBar.module.css'

const TABS = [
  { to: '/', glyph: '↑', labelKey: 'nav.tabUpload', end: true },
  { to: '/images', glyph: '⊞', labelKey: 'nav.tabImages', end: false },
  { to: '/albums', glyph: '▤', labelKey: 'nav.tabAlbums', end: false },
  { to: '/settings', glyph: '○', labelKey: 'nav.tabMine', end: false },
]

/** 移动端底部 4-Tab（<768px 显示，见 CSS）。 */
export function TabBar() {
  const { t } = useT()
  return (
    <nav className={styles.bar} data-testid="tabbar">
      {TABS.map((tab) => (
        <NavLink
          key={tab.to}
          to={tab.to}
          end={tab.end}
          className={({ isActive }) => [styles.tab, isActive && styles.active].filter(Boolean).join(' ')}
        >
          <span className={styles.glyph}>{tab.glyph}</span>
          <span className={styles.label}>{t(tab.labelKey)}</span>
        </NavLink>
      ))}
    </nav>
  )
}
