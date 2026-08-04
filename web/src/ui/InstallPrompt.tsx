import { useEffect, useState } from 'react'
import { useT } from '../i18n'

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
    <div
      className="fixed bottom-6 left-1/2 z-50 box-border flex max-w-[min(420px,calc(100vw-24px))] -translate-x-1/2 items-center gap-3 rounded border border-border bg-surface px-3.5 py-3 text-sm-plus text-ink shadow-[0_8px_28px_rgba(0,0,0,0.18)] max-md:bottom-[84px] max-md:flex-col max-md:items-stretch max-md:gap-2.5"
      role="dialog"
      aria-label={t('pwa.install')}
    >
      <span className="min-w-0 flex-1 font-semibold leading-snug text-muted">{t('pwa.installHint')}</span>
      <div className="flex flex-none items-center gap-2 max-md:justify-end">
        <button
          type="button"
          className="cursor-pointer whitespace-nowrap rounded-sm border-0 bg-btn px-3 py-2 text-sm-plus font-bold text-btn-text hover:opacity-85"
          onClick={() => {
            evt.prompt()
          }}
        >
          {t('pwa.install')}
        </button>
        <button
          type="button"
          className="cursor-pointer whitespace-nowrap border-0 bg-transparent px-1 py-2 text-xs font-semibold text-muted underline hover:text-ink"
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
