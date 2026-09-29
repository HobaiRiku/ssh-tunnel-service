// Human formatting for traffic figures. Decimal (SI) units, like most network
// tools and OS monitors show them: 1 KB = 1000 B.

const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  let value = n
  let unit = 0
  while (value >= 1000 && unit < UNITS.length - 1) {
    value /= 1000
    unit += 1
  }
  const digits = unit === 0 || value >= 100 ? 0 : value >= 10 ? 1 : 2
  return `${value.toFixed(digits)} ${UNITS[unit]}`
}

export function formatRate(bytesPerSecond: number): string {
  return `${formatBytes(bytesPerSecond)}/s`
}

// A rate is "idle" below 1 B/s — used to dim numbers that are effectively zero.
export function isIdleRate(bytesPerSecond: number): boolean {
  return !(bytesPerSecond >= 1)
}
