import { BarController, BarElement, CategoryScale, Chart, LinearScale, Tooltip } from 'chart.js'
import { Bar } from 'react-chartjs-2'
import type { DailyCount } from '../../../api/types'
import { useGlobal } from '../../../store'

Chart.register(BarController, BarElement, CategoryScale, LinearScale, Tooltip)

/** 把稀疏 daily 铺满近 days 天(本地日期,缺日补 0;默认 30)。 */
export function densify(daily: DailyCount[], days = 30): DailyCount[] {
  const byDate = new Map(daily.map((d) => [d.date, d.count]))
  const p = (n: number) => String(n).padStart(2, '0')
  const now = new Date()
  const out: DailyCount[] = []
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() - i)
    const key = `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
    out.push({ date: key, count: byDate.get(key) ?? 0 })
  }
  return out
}

export function TrendChart({ daily, days = 30 }: { daily: DailyCount[]; days?: number }) {
  const theme = useGlobal((s) => s.theme)
  const css = getComputedStyle(document.body)
  const mono = css.getPropertyValue('--font-mono').trim() || 'ui-monospace, Menlo, monospace'
  const bar = css.getPropertyValue('--btn').trim()
  const muted = css.getPropertyValue('--muted').trim()
  const border = css.getPropertyValue('--border').trim()
  const data = densify(daily, days)
  return (
    <div style={{ height: 150 }}>
      <Bar
        key={theme} /* 主题切换重挂载,重取 CSS 变量色 */
        data={{
          labels: data.map((d) => d.date.slice(5)),
          datasets: [
            {
              data: data.map((d) => d.count),
              backgroundColor: bar,
              borderRadius: 1,
              categoryPercentage: 0.9,
              barPercentage: 0.8,
            },
          ],
        }}
        options={{
          responsive: true,
          maintainAspectRatio: false,
          plugins: { tooltip: { displayColors: false } },
          scales: {
            x: {
              grid: { display: false },
              border: { color: border },
              ticks: { color: muted, maxTicksLimit: 3, maxRotation: 0, font: { size: 10, family: mono } },
            },
            y: {
              grid: { color: border },
              border: { display: false },
              ticks: { color: muted, precision: 0, maxTicksLimit: 4, font: { size: 10, family: mono } },
            },
          },
        }}
      />
    </div>
  )
}
