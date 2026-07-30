import { useEffect, useState, type RefObject } from 'react'

/** Window-scroll virtual range for a fixed-height row list (page scroll, not nested). */
export function useWindowVirtual(
  count: number,
  rowHeight: number,
  containerRef: RefObject<HTMLElement | null>,
  overscan = 10,
) {
  const [scrollY, setScrollY] = useState(() => (typeof window !== 'undefined' ? window.scrollY : 0))
  const [viewH, setViewH] = useState(() => (typeof window !== 'undefined' ? window.innerHeight : 800))
  const [top, setTop] = useState(0)

  useEffect(() => {
    const onScroll = () => setScrollY(window.scrollY)
    const onResize = () => {
      setViewH(window.innerHeight)
      const el = containerRef.current
      if (el) setTop(el.getBoundingClientRect().top + window.scrollY)
    }
    onResize()
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onResize)
    return () => {
      window.removeEventListener('scroll', onScroll)
      window.removeEventListener('resize', onResize)
    }
  }, [containerRef, count])

  // Re-measure top when count changes (layout shift from infinite scroll)
  useEffect(() => {
    const el = containerRef.current
    if (el) setTop(el.getBoundingClientRect().top + window.scrollY)
  }, [containerRef, count])

  const relTop = Math.max(0, scrollY - top)
  const start = Math.max(0, Math.floor(relTop / rowHeight) - overscan)
  const visible = Math.ceil(viewH / rowHeight) + overscan * 2
  const end = Math.min(count, start + visible)
  return {
    start,
    end,
    totalHeight: count * rowHeight,
    offsetY: start * rowHeight,
  }
}
