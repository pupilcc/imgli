import { formatBytes, formatDate } from './format'

it('formatBytes 按量级取位', () => {
  expect(formatBytes(0)).toBe('0 B')
  expect(formatBytes(512)).toBe('512 B')
  expect(formatBytes(2048)).toBe('2 KB')
  expect(formatBytes(1.8 * 1024 * 1024)).toBe('1.8 MB')
  expect(formatBytes(2.14 * 1024 ** 3)).toBe('2.14 GB')
  expect(formatBytes(10 * 1024 ** 3)).toBe('10 GB')
  expect(formatBytes(1048575)).toBe('1 MB') // KB 舍入达 1024 进位
  expect(formatBytes(1024 ** 3 - 1)).toBe('1 GB') // MB 舍入达 1024 进位
})

it('formatDate 输出 YYYY-MM-DD HH:mm', () => {
  const s = formatDate('2026-07-16T09:05:00Z')
  expect(s).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/)
})
