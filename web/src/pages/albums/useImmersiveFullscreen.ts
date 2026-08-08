import { useEffect, useRef, type RefObject } from 'react'

/** 进入沉浸时 requestFullscreen；卸载时仅 exit 本组件申请的全屏。 */
export function useImmersiveFullscreen(rootRef: RefObject<HTMLElement | null>) {
  const didRequestFs = useRef(false)

  useEffect(() => {
    const el = rootRef.current
    if (!el) return
    const anyEl = el as HTMLElement & {
      webkitRequestFullscreen?: () => Promise<void> | void
    }
    const req = el.requestFullscreen?.bind(el) || anyEl.webkitRequestFullscreen?.bind(el)
    if (!req) return
    let cancelled = false
    void Promise.resolve(req())
      .then(() => {
        if (!cancelled) didRequestFs.current = true
      })
      .catch(() => {
        /* 无手势 / 策略拒绝 — 仍保留沉浸 UI */
      })
    return () => {
      cancelled = true
      if (!didRequestFs.current) return
      didRequestFs.current = false
      const doc = document as Document & {
        webkitExitFullscreen?: () => Promise<void> | void
        webkitFullscreenElement?: Element | null
      }
      const fsEl = document.fullscreenElement || doc.webkitFullscreenElement
      if (!fsEl) return
      const exit = document.exitFullscreen?.bind(document) || doc.webkitExitFullscreen?.bind(document)
      if (exit) void Promise.resolve(exit()).catch(() => {})
    }
  }, [rootRef])
}
