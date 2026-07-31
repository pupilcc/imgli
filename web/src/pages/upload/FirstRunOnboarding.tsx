import { Link } from 'react-router'
import { useT } from '../../i18n'
import styles from './FirstRunOnboarding.module.css'

const KEY = 'imgli_onboarding_dismissed'

export function FirstRunOnboarding({ show }: { show: boolean }) {
  const { t } = useT()
  if (!show) return null
  if (typeof localStorage !== 'undefined' && localStorage.getItem(KEY) === '1') return null

  function dismiss() {
    try {
      localStorage.setItem(KEY, '1')
    } catch {
      /* ignore */
    }
    // force re-render by hiding via location reload-free: replace node
    const el = document.getElementById('imgli-first-run')
    el?.remove()
  }

  return (
    <aside id="imgli-first-run" className={styles.card} role="note">
      <div className={styles.kicker}>{t('upload.onboardingKicker')}</div>
      <ol className={styles.steps}>
        <li>{t('upload.onboardingStep1')}</li>
        <li>{t('upload.onboardingStep2')}</li>
        <li>{t('upload.onboardingStep3')}</li>
      </ol>
      <div className={styles.actions}>
        <Link className={styles.primary} to="/settings">
          {t('upload.onboardingCta')}
        </Link>
        <button type="button" className={styles.ghost} onClick={dismiss}>
          {t('upload.onboardingSkip')}
        </button>
      </div>
    </aside>
  )
}
