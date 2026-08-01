import { useState } from 'react'
import { Link, NavLink, Outlet } from 'react-router'
import { useReviewCount } from '../../../api/adminHooks'
import { useSession } from '../../../api/hooks'
import { useT } from '../../../i18n'
import { BrandLockup } from '../../../ui/Brand'
import { LangToggle } from '../../../ui/LangToggle'
import { useGlobal } from '../../../store'
import styles from './AdminLayout.module.css'

const NAV = [
  { to: '/admin', glyph: '◱', key: 'dashboard' as const, end: true },
  { to: '/admin/users', glyph: '◔', key: 'users' as const },
  { to: '/admin/images', glyph: '▦', key: 'images' as const },
  { to: '/admin/review', glyph: '◈', key: 'review' as const },
  { to: '/admin/groups', glyph: '◫', key: 'groups' as const },
  { to: '/admin/invites', glyph: '◇', key: 'invites' as const },
  { to: '/admin/policies', glyph: '▤', key: 'policies' as const },
  { to: '/admin/system', glyph: '◎', key: 'systemOps' as const },
  { to: '/admin/settings', glyph: '⚙', key: 'systemSettings' as const },
  { to: '/admin/logs', glyph: '≡', key: 'logs' as const },
]

export function AdminLayout() {
  const { t } = useT()
  const { data: user } = useSession()
  const theme = useGlobal((s) => s.theme)
  const toggleTheme = useGlobal((s) => s.toggleTheme)
  const review = useReviewCount()
  const [drawer, setDrawer] = useState(false)
  if (!user) return null

  const nav = (
    <nav className={styles.nav}>
      <div className={styles.consoleLabel}>CONSOLE</div>
      {NAV.map((n) => (
        <NavLink
          key={n.to}
          to={n.to}
          end={n.end}
          className={({ isActive }) => [styles.item, isActive && styles.itemActive].filter(Boolean).join(' ')}
          onClick={() => setDrawer(false)}
        >
          <span className={styles.glyph}>{n.glyph}</span>
          {t(`nav.${n.key}`)}
          {n.to === '/admin/review' && !!review.data && <span className={styles.badge}>{review.data}</span>}
        </NavLink>
      ))}
    </nav>
  )

  return (
    <div className={styles.root}>
      <header className={styles.header}>
        <button type="button" className={styles.burger} aria-label={t('nav.menu')} onClick={() => setDrawer((v) => !v)}>
          ≡
        </button>
        <Link to="/admin" className={styles.brand} aria-label={t('nav.adminHomeAria')}>
          <BrandLockup badge="ADMIN" />
        </Link>
        <div className={styles.right}>
          <Link to="/" className={styles.backLink}>
            {t('nav.backToConsole')}
          </Link>
          <button type="button" className={styles.themeBtn} title={t('nav.toggleTheme')} onClick={toggleTheme}>
            {theme === 'light' ? '◐' : '◑'}
          </button>
          <LangToggle />
          <div className={styles.avatar}>{(user.nickname || user.username).slice(0, 1)}</div>
        </div>
      </header>
      <div className={styles.body}>
        <aside className={styles.sidebar}>{nav}</aside>
        {drawer && (
          <div className={styles.drawerWrap} onClick={() => setDrawer(false)}>
            <aside className={styles.drawer} onClick={(e) => e.stopPropagation()}>
              {nav}
            </aside>
          </div>
        )}
        <main className={styles.main}>
          <Outlet />
        </main>
      </div>
    </div>
  )
}
