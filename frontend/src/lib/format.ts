const nf1 = new Intl.NumberFormat('pt-BR', { minimumFractionDigits: 1, maximumFractionDigits: 1 })
const nf0 = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 0 })
const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

/** Base 1024, one decimal place, pt-BR separators ("1,5 GB"). */
export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  if (i === 0) return `${nf0.format(v)} B`
  // Rounding can reach 1024.0 (e.g. 1023.99 KB); promote to the next unit.
  if (Math.round(v * 10) / 10 >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${nf1.format(v)} ${units[i]}`
}

/** Unix seconds -> dd/mm/aaaa (local time). */
export function formatDate(unixSec: number): string {
  const d = new Date(unixSec * 1000)
  const p = (x: number) => String(x).padStart(2, '0')
  return `${p(d.getDate())}/${p(d.getMonth() + 1)}/${d.getFullYear()}`
}

/** Integer with pt-BR thousands separators ("1.234.567"). */
export function formatCount(n: number): string {
  return nf0.format(n)
}
