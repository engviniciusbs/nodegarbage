import type { VolumeDTO } from '$lib/api'

/** Unix ms of the volume's saved scan, or null when it was never scanned (Go zero time included). */
export function scanTime(v: Pick<VolumeDTO, 'lastScan'>): number | null {
  const t = Date.parse(String(v.lastScan ?? ''))
  return Number.isFinite(t) && new Date(t).getFullYear() > 2000 ? t : null
}

/** Root of the most recently scanned volume (first scanned one on ties), or '' when none has a scan. */
export function pickScannedVolume(vols: Pick<VolumeDTO, 'root' | 'lastScan'>[]): string {
  let best = ''
  let bestT = -Infinity
  for (const v of vols ?? []) {
    const t = scanTime(v)
    if (t !== null && t > bestT) {
      best = v.root
      bestT = t
    }
  }
  return best
}
