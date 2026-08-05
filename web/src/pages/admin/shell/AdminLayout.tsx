import { useState } from 'react'
import { Link, NavLink, Outlet } from 'react-router'
import { useReviewCount } from '../../../api/adminHooks'
import { useConfig, useSession } from '../../../api/hooks'
import { useT } from '../../../i18n'
import { cn } from '../../../lib/cn'
import { useGlobal } from '../../../store'
import { BrandLockup } from '../../../ui/Brand'
import { LangToggle } from '../../../ui/LangToggle'

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

const themeBtn =
  'flex h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-sm border border-border bg-surface text-sm text-ink hover:bg-soft'

export function AdminLayout() {
  const { t } = useT()
  const { data: user } = useSession()
  const { data: config } = useConfig()
  const theme = useGlobal((s) => s.theme)
  const toggleTheme = useGlobal((s) => s.toggleTheme)
  const review = useReviewCount()
  const [drawer, setDrawer] = useState(false)
  if (!user) return null

  const nav = (
    <nav className="flex flex-col gap-0.5">
      <div className="mb-2.5 px-2.5 font-mono text-2xs tracking-[0.14em] text-muted">CONSOLE</div>
      {NAV.map((n) => (
        <NavLink
          key={n.to}
          to={n.to}
          end={n.end}
          className={({ isActive }) =>
            cn(
              'flex items-center gap-[9px] rounded-sm px-2.5 py-[9px] text-[13px] font-semibold text-muted transition-colors duration-150 hover:bg-soft hover:text-ink',
              isActive && 'bg-btn text-btn-text hover:bg-btn hover:text-btn-text',
            )
          }
          onClick={() => setDrawer(false)}
        >
          <span className="w-3.5 font-mono text-[11px]">{n.glyph}</span>
          {t(`nav.${n.key}`)}
          {n.to === '/admin/review' && !!review.data && (
            <span className="ml-auto rounded-sm bg-err px-1.5 py-px font-mono text-2xs text-white">{review.data}</span>
          )}
        </NavLink>
      ))}
    </nav>
  )

  return (
    <div className="flex h-dvh flex-col overflow-hidden bg-bg">
      <header className="z-20 flex h-14 flex-none items-center gap-4 border-b border-border bg-surface px-8 max-md:px-4">
        <button
          type="button"
          className="hidden h-[30px] w-[30px] cursor-pointer items-center justify-center rounded-sm border border-border bg-surface text-sm text-ink max-md:flex"
          aria-label={t('nav.menu')}
          onClick={() => setDrawer((v) => !v)}
        >
          ≡
        </button>
        <Link to="/admin" className="flex items-center text-ink" aria-label={t('nav.adminHomeAria')}>
          <BrandLockup badge="ADMIN" word={config?.site_name} />
        </Link>
        <div className="ml-auto flex items-center gap-3.5">
          <Link to="/" className="text-sm-plus font-semibold text-muted hover:text-ink">
            {t('nav.backToConsole')}
          </Link>
          <button type="button" className={themeBtn} title={t('nav.toggleTheme')} onClick={toggleTheme}>
            {theme === 'light' ? '◐' : '◑'}
          </button>
          <LangToggle />
          <div className="flex h-[30px] w-[30px] items-center justify-center rounded-full border border-border bg-soft text-xs font-bold">
            {(user.nickname || user.username).slice(0, 1)}
          </div>
        </div>
      </header>
      <div className="mx-auto box-border flex min-h-0 w-full max-w-[1320px] flex-1 overflow-hidden">
        <aside className="box-border h-full w-[196px] flex-none overflow-y-auto overscroll-contain border-r border-border bg-bg px-4 py-7 max-md:hidden">
          {nav}
        </aside>
        {drawer && (
          <div className="fixed inset-x-0 top-14 bottom-0 z-20 bg-black/35" onClick={() => setDrawer(false)}>
            <aside
              className="box-border h-full w-[220px] overflow-y-auto border-r border-border bg-bg px-4 py-7"
              onClick={(e) => e.stopPropagation()}
            >
              {nav}
            </aside>
          </div>
        )}
        <main className="box-border min-h-0 min-w-0 flex-1 overflow-y-auto overscroll-contain scroll-pt-3 px-9 pt-7 pb-20 max-md:px-4 max-md:pt-5 max-md:pb-16">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
