import { Link, Outlet } from 'react-router'
import { useConfig } from '../api/hooks'
import { useT } from '../i18n'
import { loginHref } from '../lib/safeNext'
import { BrandLockup } from '../ui/Brand'
import { LangToggle } from '../ui/LangToggle'
import { SiteFooter } from '../ui/SiteSlots'
import { useGlobal } from '../store'
import styles from './GuestLayout.module.css'

/** 未登录精简布局：品牌 + 主题 + 登录入口；可看上传页（是否可传由后端与 UploadPage 提示）。 */
export function GuestLayout() {
  const { t } = useT()
  const theme = useGlobal((s) => s.theme)
  const toggleTheme = useGlobal((s) => s.toggleTheme)
  const { data: config } = useConfig()
  const guestOn = !!config?.guest_upload_enabled
  return (
    <div className={styles.shell}>
      <header className={styles.nav}>
        <span className={styles.brand} aria-label="img.li">
          <BrandLockup badge={guestOn ? 'GUEST' : undefined} />
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
          <Link to={loginHref('/')} className={styles.loginBtn}>
            {guestOn ? t('nav.loginToManage') : t('nav.loginOrRegister')}
          </Link>
        </div>
      </header>
      <main className={styles.main}>
        <Outlet />
      </main>
      <SiteFooter
        footer={config?.footer}
        siteName={config?.site_name}
        ossCredit={config?.oss_credit}
        sourceUrl={config?.source_url}
        aboutEnabled={!!config?.about_enabled}
      />
    </div>
  )
}
