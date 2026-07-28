import { useEffect, useState } from 'react'
import { useT } from '../i18n'
import styles from './InstallPrompt.module.css'

const DISMISS_KEY = 'imgli-pwa-dismissed'
type BIPEvent = Event & { prompt: () => Promise<void>; userChoice: Promise<{ outcome: string }> }

export function InstallPrompt() {
  const { t } = useT()
  const [evt, setEvt] = useState<BIPEvent | null>(null)
  const [hidden, setHidden] = useState(() => localStorage.getItem(DISMISS_KEY) === '1')

  useEffect(() => {
    const onBIP = (e: Event) => {
      e.preventDefault()
      setEvt(e as BIPEvent)
    }
    const onInstalled = () => {
      setEvt(null)
      setHidden(true)
    }
    window.addEventListener('beforeinstallprompt', onBIP)
    window.addEventListener('appinstalled', onInstalled)
    return () => {
      window.removeEventListener('beforeinstallprompt', onBIP)
      window.removeEventListener('appinstalled', onInstalled)
    }
  }, [])

  if (hidden || !evt) return null
  return (
    <div className={styles.bar} role="dialog" aria-label={t('pwa.install')}>
      <span className={styles.hint}>{t('pwa.installHint')}</span>
      <div className={styles.actions}>
        <button type="button" className={styles.install} onClick={() => { evt.prompt() }}>
          {t('pwa.install')}
        </button>
        <button
          type="button"
          className={styles.dismiss}
          onClick={() => {
            localStorage.setItem(DISMISS_KEY, '1')
            setHidden(true)
          }}
        >
          {t('pwa.dismiss')}
        </button>
      </div>
    </div>
  )
}
