/** 胶片条窗口：只渲染 index 附近切片，两侧用 spacer 保持滚动宽度。 */

export type FilmstripWindow = {
  start: number
  end: number // exclusive
  padLeft: number
  padRight: number
}

/**
 * @param count 总张数
 * @param index 当前 0-based
 * @param half 单侧缓冲张数（窗口约 2*half+1）
 * @param tile 每格占用宽度（含 gap）px
 */
export function filmstripWindow(
  count: number,
  index: number,
  half = 24,
  tile = 62,
): FilmstripWindow {
  if (count <= 0) return { start: 0, end: 0, padLeft: 0, padRight: 0 }
  const i = Math.min(Math.max(0, index), count - 1)
  // 小列表全量渲染，避免无意义 spacer
  if (count <= half * 2 + 1) {
    return { start: 0, end: count, padLeft: 0, padRight: 0 }
  }
  let start = Math.max(0, i - half)
  let end = Math.min(count, i + half + 1)
  // 贴边时尽量吃满窗口宽度
  const want = half * 2 + 1
  if (end - start < want) {
    if (start === 0) end = Math.min(count, want)
    else if (end === count) start = Math.max(0, count - want)
  }
  return {
    start,
    end,
    padLeft: start * tile,
    padRight: (count - end) * tile,
  }
}

/** 距离列表末尾 ≤ threshold 时建议预取下一页 */
export function shouldPrefetchMore(
  index: number,
  loadedCount: number,
  hasNextPage: boolean,
  threshold = 6,
): boolean {
  if (!hasNextPage || loadedCount <= 0) return false
  return index >= loadedCount - threshold
}
