import { useGlobal } from '../store'

export function Toasts() {
  const toasts = useGlobal((s) => s.toasts)
  if (toasts.length === 0) return null
  return (
    <div
      className="pointer-events-none fixed bottom-7 left-1/2 z-[100] flex -translate-x-1/2 flex-col items-center gap-2 max-md:bottom-[84px]"
      aria-live="polite"
    >
      {toasts.map((t) => (
        <div
          key={t.id}
          className="animate-[toastUp_0.2s] rounded-sm bg-btn px-[18px] py-[9px] text-sm-plus font-bold text-btn-text shadow-[0_8px_28px_rgba(0,0,0,0.25)]"
        >
          {t.message}
        </div>
      ))}
    </div>
  )
}
