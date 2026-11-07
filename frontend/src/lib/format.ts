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

/** Unix seconds -> hh:mm (local time, 24 h). */
export function formatTime(unixSec: number): string {
  const d = new Date(unixSec * 1000)
  const p = (x: number) => String(x).padStart(2, '0')
  return `${p(d.getHours())}:${p(d.getMinutes())}`
}

/** Milliseconds -> "2 min 41 s", "45 s", "1 h 3 min". */
export function formatDuration(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) return m > 0 ? `${h} h ${m} min` : `${h} h`
  if (m > 0) return sec > 0 ? `${m} min ${sec} s` : `${m} min`
  return `${sec} s`
}
