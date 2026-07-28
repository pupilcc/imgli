/** 字节数 → 人类可读：B 整数；KB/MB 一位小数；GB 两位小数（去尾零）。 */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  const kb = n / 1024
  const kbR = Math.round(kb * 10) / 10
  if (kbR < 1024) return `${kbR} KB`
  const mb = kb / 1024
  const mbR = Math.round(mb * 10) / 10
  if (mbR < 1024) return `${mbR} MB`
  return `${parseFloat((mb / 1024).toFixed(2))} GB`
}

/** RFC3339 → 本地时区 `YYYY-MM-DD HH:mm`。 */
export function formatDate(iso: string): string {
  const d = new Date(iso)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}
