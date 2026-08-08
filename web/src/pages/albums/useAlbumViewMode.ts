import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router'
import {
  buildAlbumSearch,
  parseIndexParam,
  type AlbumPublicMode,
} from './albumPublicView'
import { shouldPrefetchMore } from './filmstripWindow'

type Opts = {
  defaultView?: string
  rowsLen: number
  hasNextPage: boolean
  isFetchingNextPage: boolean
  fetchNextPage: () => void
  /** 元数据与首批图片就绪后才 bootstrap default_view */
  ready: boolean
}

/**
 * 公开相册视图：URL 优先（view/i），无访客 localStorage。
 * 无显式 view 时采用属主 default_view。
 */
export function useAlbumViewMode({
  defaultView,
  rowsLen,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
  ready,
}: Opts) {
  const [searchParams, setSearchParams] = useSearchParams()
  const viewFromUrl = searchParams.get('view') === 'immersive' ? 'immersive' : 'gallery'
  const indexFromUrl = parseIndexParam(searchParams.get('i'))

  const [activeIndex, setActiveIndex] = useState<number | null>(() =>
    viewFromUrl === 'immersive' ? indexFromUrl : null,
  )

  const awaitAdvanceRef = useRef(false)
  const rowsLenRef = useRef(0)
  const hasNextPageRef = useRef(false)
  const fetchingMoreRef = useRef(false)
  const fetchNextPageRef = useRef<() => void>(() => {})
  const bootstrapped = useRef(false)

  rowsLenRef.current = rowsLen
  hasNextPageRef.current = hasNextPage
  fetchingMoreRef.current = isFetchingNextPage
  fetchNextPageRef.current = fetchNextPage

  const immersive = activeIndex != null
  const mode: AlbumPublicMode = immersive ? 'immersive' : 'gallery'

  const syncUrl = useCallback(
    (nextMode: AlbumPublicMode, index0: number) => {
      const q = buildAlbumSearch(nextMode, index0)
      if (!q) {
        if (searchParams.toString()) setSearchParams({}, { replace: true })
        return
      }
      const next = new URLSearchParams(q)
      if (searchParams.toString() !== next.toString()) {
        setSearchParams(next, { replace: true })
      }
    },
    [searchParams, setSearchParams],
  )

  const openImmersive = useCallback(
    (index0: number) => {
      const i = Math.max(0, index0)
      setActiveIndex(i)
      syncUrl('immersive', i)
    },
    [syncUrl],
  )

  const closeImmersive = useCallback(() => {
    awaitAdvanceRef.current = false
    setActiveIndex(null)
    syncUrl('gallery', 0)
  }, [syncUrl])

  // URL → 状态（前进后退 / 外链）
  useEffect(() => {
    if (viewFromUrl === 'immersive') {
      setActiveIndex((cur) => (cur === indexFromUrl ? cur : indexFromUrl))
    } else if (activeIndex != null) {
      setActiveIndex(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentional URL-driven
  }, [viewFromUrl, indexFromUrl])

  // 深链 i 超出已加载：持续拉页；末尾钳位
  useEffect(() => {
    if (activeIndex == null) return
    if (activeIndex < rowsLen) return
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage()
    } else if (!hasNextPage && rowsLen > 0 && activeIndex >= rowsLen) {
      openImmersive(rowsLen - 1)
    }
  }, [activeIndex, rowsLen, hasNextPage, isFetchingNextPage, fetchNextPage, openImmersive])

  // 无显式 URL 时：属主 default_view
  useEffect(() => {
    if (bootstrapped.current || !ready) return
    if (searchParams.get('view') === 'immersive') {
      bootstrapped.current = true
      return
    }
    bootstrapped.current = true
    if (defaultView === 'immersive' && activeIndex == null) {
      openImmersive(0)
    }
  }, [ready, defaultView, searchParams, activeIndex, openImmersive])

  const goPrev = useCallback(() => {
    setActiveIndex((i) => (i != null && i > 0 ? i - 1 : i))
  }, [])

  const goNext = useCallback(() => {
    setActiveIndex((i) => {
      if (i == null) return i
      if (i < rowsLenRef.current - 1) return i + 1
      if (hasNextPageRef.current && !fetchingMoreRef.current) {
        awaitAdvanceRef.current = true
        fetchNextPageRef.current()
      }
      return i
    })
  }, [])

  // 翻页完成后前进
  useEffect(() => {
    if (!awaitAdvanceRef.current || activeIndex == null) return
    if (activeIndex < rowsLen - 1) {
      awaitAdvanceRef.current = false
      setActiveIndex(activeIndex + 1)
    } else if (!isFetchingNextPage && !hasNextPage) {
      awaitAdvanceRef.current = false
    }
  }, [rowsLen, activeIndex, isFetchingNextPage, hasNextPage])

  // 沉浸 index 变化时同步 URL
  useEffect(() => {
    if (activeIndex == null) return
    syncUrl('immersive', activeIndex)
  }, [activeIndex, syncUrl])

  const selectIndex = useCallback((i: number) => {
    setActiveIndex(i)
  }, [])

  // 键盘 + body scroll lock
  useEffect(() => {
    if (!immersive) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') closeImmersive()
      else if (e.key === 'ArrowLeft') goPrev()
      else if (e.key === 'ArrowRight') goNext()
    }
    document.addEventListener('keydown', onKey)
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prevOverflow
    }
  }, [immersive, closeImmersive, goPrev, goNext])

  // 沉浸接近末尾预取
  useEffect(() => {
    if (activeIndex == null) return
    if (shouldPrefetchMore(activeIndex, rowsLen, hasNextPage) && !isFetchingNextPage) {
      fetchNextPage()
    }
  }, [activeIndex, rowsLen, hasNextPage, isFetchingNextPage, fetchNextPage])

  return {
    activeIndex,
    mode,
    immersive,
    openImmersive,
    closeImmersive,
    goPrev,
    goNext,
    selectIndex,
  }
}
