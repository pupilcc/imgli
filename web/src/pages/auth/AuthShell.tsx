import type { ReactNode } from 'react'
import { useT } from '../../i18n'
import { BrandLockup } from '../../ui/Brand'
import { LangToggle } from '../../ui/LangToggle'
import { useGlobal } from '../../store'
import styles from './AuthPage.module.css'

/** 登录/注册与邮件流公开页共用的品牌壳(左 brand aside + 右表单 pane)。 */
export function AuthShell({ children }: { children: ReactNode }) {
  const { t } = useT()
  const theme = useGlobal((s) => s.theme)
  const toggleTheme = useGlobal((s) => s.toggleTheme)
  return (
    <div className={styles.page}>
      <aside className={styles.brand}>
        <div className={styles.brandLogo}>
          <BrandLockup beta invert />
        </div>
        <div>
          <div className={styles.slogan}>{t('meta.slogan')}</div>
          <div className={styles.headline}>
            {t('auth.headlineLine1')}
            <br />
            {t('auth.headlineLine2')}
          </div>
        </div>
        <div className={styles.copyright}>
          {t('auth.copyright', { year: new Date().getFullYear() })}
        </div>
        <div className={styles.deco} />
      </aside>
      <main className={styles.formPane}>
        <div className={styles.topBar}>
          <button type="button" className={styles.themeBtn} title={t('nav.toggleTheme')} onClick={toggleTheme}>
            {theme === 'light' ? '◐' : '◑'}
          </button>
          <LangToggle />
        </div>
        <div className={styles.formBox}>{children}</div>
      </main>
    </div>
  )
}
