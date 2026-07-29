import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, NavLink, useNavigate } from 'react-router'
import { useConfig, useLogout, useQuota } from '../api/hooks'
import type { User } from '../api/types'
import { useT } from '../i18n'
import { useGlobal } from '../store'
import { BrandLockup } from '../ui/Brand'
import { LangToggle } from '../ui/LangToggle'
import { NavQuotaCluster } from '../ui/QuotaBar'
import styles from './Nav.module.css'

const baseLinks: { to: string; key: string }[] = [
  { to: '/', key: 'upload' },
  { to: '/images', key: 'myImages' },
  { to: '/albums', key: 'albums' },
  { to: '/settings', key: 'settings' },
]

export function Nav({ user }: { user: User }) {
  const { t, lang } = useT()
  const theme = useGlobal((s) => s.theme)
  const toggleTheme = useGlobal((s) => s.toggleTheme)
  const { data: config } = useConfig()
  const quota = useQuota()
  const logout = useLogout()
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const [imgFailed, setImgFailed] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  const links = useMemo(() => {
    const withLabel = baseLinks.map((l) => ({ ...l, label: t(`nav.${l.key}`) }))
    if (!config?.plaza_enabled) return withLabel
    // 广场插在相册之后、设置之前
    const i = withLabel.findIndex((l) => l.to === '/settings')
    const next = [...withLabel]
    next.splice(i >= 0 ? i : next.length, 0, { to: '/explore', key: 'plaza', label: t('nav.plaza') })
    return next
  }, [config?.plaza_enabled, lang, t])

  // 头像换了(上传新头像/换账号)要重试加载,失败态不跨 URL 粘滞(codex 终审)
  useEffect(() => {
    setImgFailed(false)
  }, [user.avatar_url])

  useEffect(() => {
    if (!menuOpen) return
    const close = (e: MouseEvent) => {
      if (!menuRef.current?.contains(e.target as Node)) setMenuOpen(false)
    }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [menuOpen])

  return (
    <header className={styles.nav}>
      <Link to="/" className={styles.brand} aria-label={t('nav.homeAria')}>
        <BrandLockup beta />
      </Link>
      <nav className={styles.links}>
        {links.map((l) => (
          <NavLink
            key={l.to}
            to={l.to}
            end={l.to === '/'}
            className={({ isActive }) => (isActive ? styles.active : undefined)}
          >
            {l.label}
          </NavLink>
        ))}
      </nav>
      <div className={styles.right}>
        {quota.data && (
          <NavQuotaCluster
            storage={{ used: quota.data.used, total: quota.data.total }}
            bandwidth={
              (quota.data.bandwidth_quota_month ?? 0) > 0
                ? {
                    used: quota.data.bandwidth_used_month ?? 0,
                    total: quota.data.bandwidth_quota_month ?? 0,
                  }
                : null
            }
          />
        )}
        <button
          type="button"
          className={styles.themeBtn}
          title={t('nav.toggleTheme')}
          onClick={toggleTheme}
        >
          {theme === 'light' ? '◐' : '◑'}
        </button>
        <LangToggle />
        <div className={styles.avatarWrap} ref={menuRef}>
          <button type="button" className={styles.avatar} onClick={() => setMenuOpen((v) => !v)}>
            {user.avatar_url && !imgFailed ? (
              <img className={styles.avatarImg} src={user.avatar_url} alt="" onError={() => setImgFailed(true)} />
            ) : (
              (user.nickname || user.username).slice(0, 1)
            )}
          </button>
          {menuOpen && (
            <div className={styles.menu}>
              <div className={styles.menuUser}>
                <div className={styles.menuName}>{user.nickname || user.username}</div>
                <div className={styles.menuEmail}>{user.email}</div>
              </div>
              {user.is_admin && (
                <Link to="/admin" className={styles.menuItem} onClick={() => setMenuOpen(false)}>
                  {t('nav.admin')}
                </Link>
              )}
              <Link to="/settings" className={styles.menuItem} onClick={() => setMenuOpen(false)}>
                {t('nav.settings')}
              </Link>
              <button
                type="button"
                className={`${styles.menuItem} ${styles.menuDanger}`}
                onClick={() => logout.mutate(undefined, { onSuccess: () => navigate('/login') })}
              >
                {t('nav.logout')}
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  )
}
