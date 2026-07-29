import { Link, Outlet } from 'react-router'
import { useConfig } from '../api/hooks'
import { useT } from '../i18n'
import { BrandLockup } from '../ui/Brand'
import { LangToggle } from '../ui/LangToggle'
import { SiteFooter } from '../ui/SiteSlots'
import { useGlobal } from '../store'
import styles from './GuestLayout.module.css'

/** 游客模式精简布局：品牌 + 主题切换 + 登录入口，无图库/相册/设置导航。 */
export function GuestLayout() {
  const { t } = useT()
  const theme = useGlobal((s) => s.theme)
  const toggleTheme = useGlobal((s) => s.toggleTheme)
  const { data: config } = useConfig()
  return (
    <>
      <header className={styles.nav}>
        <span className={styles.brand} aria-label="img.li">
          <BrandLockup badge="GUEST" />
        </span>
        <div className={styles.right}>
          <button type="button" className={styles.themeBtn} title={t('nav.toggleTheme')} onClick={toggleTheme}>
            {theme === 'light' ? '◐' : '◑'}
          </button>
          <LangToggle />
          {config?.plaza_enabled && (
            <Link to="/explore" className={styles.plazaLink}>
              {t('nav.plaza')}
            </Link>
          )}
          <Link to="/login" className={styles.loginBtn}>
            {t('nav.loginToManage')}
          </Link>
        </div>
      </header>
      <main className={styles.main}>
        <Outlet />
      </main>
      <SiteFooter footer={config?.footer} />
    </>
  )
}
