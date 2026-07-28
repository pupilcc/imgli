import { Link, Outlet } from 'react-router'
import { useSession } from '../../api/hooks'
import { useT } from '../../i18n'
import { BrandLockup } from '../../ui/Brand'
import { LangToggle } from '../../ui/LangToggle'
import { useGlobal } from '../../store'
import styles from './DiscoverLayout.module.css'

/** 发现面公开壳：品牌 + 主题 + 登录态入口，子路由经 Outlet 渲染。 */
export function DiscoverLayout() {
  const { t } = useT()
  const theme = useGlobal((s) => s.theme)
  const toggleTheme = useGlobal((s) => s.toggleTheme)
  const { data: user } = useSession()

  return (
    <>
      <header className={styles.nav}>
        <Link to="/explore" className={styles.brand} aria-label="img.li">
          <BrandLockup />
        </Link>
        <div className={styles.right}>
          <button type="button" className={styles.themeBtn} title={t('nav.toggleTheme')} onClick={toggleTheme}>
            {theme === 'light' ? '◐' : '◑'}
          </button>
          <LangToggle />
          {user ? (
            <Link to="/" className={styles.linkBtn}>
              {t('nav.backToUpload')}
            </Link>
          ) : (
            <>
              <Link to="/login" className={styles.textLink}>
                {t('nav.login')}
              </Link>
              <Link to="/login" className={styles.linkBtn}>
                {t('nav.register')}
              </Link>
            </>
          )}
        </div>
      </header>
      <main className={styles.main}>
        <Outlet />
      </main>
    </>
  )
}
