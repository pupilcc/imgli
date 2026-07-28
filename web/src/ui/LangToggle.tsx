import { useT } from '../i18n'
import { useSession, useUpdatePreferences } from '../api/hooks'
import styles from './LangToggle.module.css'

/**
 * 中/EN 语言切换。匿名仅写 localStorage(setLang);登录态额外把语言写入 Preferences.lang
 * 实现跨设备同步——偏好为全量替换,故合并当前完整偏好再覆盖 lang,避免清空其它偏好。
 */
export function LangToggle() {
  const { lang, setLang } = useT()
  const { data: session } = useSession()
  const updatePrefs = useUpdatePreferences()
  const toggle = () => {
    const next = lang === 'zh' ? 'en' : 'zh'
    setLang(next)
    if (session?.preferences) updatePrefs.mutate({ ...session.preferences, lang: next })
  }
  return (
    <button
      type="button"
      className={styles.btn}
      title={lang === 'zh' ? 'Switch to English' : '切换到中文'}
      aria-label="language"
      onClick={toggle}
    >
      {lang === 'zh' ? 'EN' : '中'}
    </button>
  )
}
