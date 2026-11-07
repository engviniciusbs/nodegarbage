import { StartScan, CancelScan, on } from '$lib/api'
import type { ProgressDTO, ScanDoneDTO } from '$lib/api'
import { route } from '$lib/route.svelte'

export const CANCELED = 'canceled'

export const scanState = $state({
  running: false,
  queue: [] as string[],
  progress: {} as Record<string, ProgressDTO>,
  done: {} as Record<string, ScanDoneDTO>,
  startError: '',
})

let unsubs: Array<() => void> = []

/** Subscribes to scan events once; safe to call repeatedly. */
export function initScanEvents(): void {
  if (unsubs.length) return
  unsubs = [
    on('scan:progress', (p: ProgressDTO) => {
      scanState.progress[p.volume] = p
    }),
    on('scan:done', onDone),
  ]
}

function onDone(d: ScanDoneDTO): void {
  scanState.done[d.volume] = d
  const canceled = d.error === CANCELED
  const finished = scanState.queue.every((r) => scanState.done[r])
  if (canceled || finished) scanState.running = false
  if (!finished || canceled) return
  const anyError = scanState.queue.some((r) => scanState.done[r].error)
  // A warning (e.g. the snapshot was not saved) does not block the results:
  // the Scan screen shows it with a button to open them.
  const anyWarning = scanState.queue.some((r) => scanState.done[r].warning)
  if (!anyError && !anyWarning) {
    route.volume = scanState.queue[0]
    route.current = 'results'
  }
}

export function resetScan(): void {
  scanState.running = false
  scanState.queue = []
  scanState.progress = {}
  scanState.done = {}
  scanState.startError = ''
}

export async function startScan(roots: string[]): Promise<void> {
  initScanEvents()
  resetScan()
  scanState.queue = [...roots]
  scanState.running = true
  try {
    await StartScan(roots)
  } catch (e) {
    scanState.running = false
    scanState.startError = e instanceof Error ? e.message : String(e)
    throw e
  }
}

export async function cancelScan(): Promise<void> {
  await CancelScan()
}
