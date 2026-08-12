import type { PublicStatsSnapshot } from '../../api/types'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { formatBytes } from '../../lib/format'

/** 游客首页可选的实例公开统计条；仅当 public_stats.enabled 且至少一项有值时渲染。 */
export function InstanceStatsBar({
  stats,
  className,
}: {
  stats?: PublicStatsSnapshot | null
  className?: string
}) {
  const { t } = useT()
  if (!stats?.enabled) return null

  const items: { key: string; label: string; value: string }[] = []
  if (stats.uptime_days != null && stats.uptime_days >= 0) {
    items.push({
      key: 'days',
      label: t('upload.statsUptime'),
      value: t('upload.statsDays', { n: stats.uptime_days }),
    })
  }
  if (stats.live_image_count != null) {
    items.push({
      key: 'images',
      label: t('upload.statsImages'),
      value: formatCount(stats.live_image_count),
    })
  }
  if (stats.user_count != null) {
    items.push({
      key: 'users',
      label: t('upload.statsUsers'),
      value: formatCount(stats.user_count),
    })
  }
  if (stats.used_bytes != null && stats.used_bytes >= 0) {
    items.push({
      key: 'bytes',
      label: t('upload.statsStorage'),
      value: formatBytes(stats.used_bytes),
    })
  }
  if (items.length === 0) return null

  // 单行指标条 + 竖分割线：比 4 列卡片网格更贴合现有「surface + mono 标签」语言，也不抢标题层级。
  return (
    <div
      className={cn(
        'flex flex-wrap overflow-hidden rounded border border-border bg-surface',
        className,
      )}
      data-testid="instance-stats"
      aria-label={t('upload.statsAria')}
    >
      {items.map((it, i) => (
        <div
          key={it.key}
          className={cn(
            'flex min-w-[7.5rem] flex-1 flex-col justify-center px-4 py-3',
            i > 0 && 'border-l border-border max-sm:border-l-0 max-sm:border-t',
          )}
        >
          <div className="font-mono text-[10px] font-semibold tracking-[0.12em] text-muted uppercase">
            {it.label}
          </div>
          <div className="mt-1 truncate text-[14px] font-bold tabular-nums tracking-[-0.01em] text-ink">
            {it.value}
          </div>
        </div>
      ))}
    </div>
  )
}

function formatCount(n: number): string {
  if (!Number.isFinite(n) || n < 0) return '0'
  try {
    return new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(n)
  } catch {
    return String(Math.floor(n))
  }
}
