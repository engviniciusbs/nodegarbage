import {
  ListAIProfiles,
  GetAIReport,
  PreviewAIPayload,
  RunAI,
  CancelAI,
  LocatePath,
  on,
} from '$lib/api'
import type { AIProfileDTO, AIReportDTO, PayloadPreviewDTO } from '$lib/api'
import { route } from '$lib/route.svelte'
import { revealPath } from './results.svelte'
import { strings } from '$lib/strings'

export const ai = $state({
  profiles: [] as AIProfileDTO[],
  profileId: '',
  report: null as AIReportDTO | null,
  preview: null as PayloadPreviewDTO | null,
  previewLoading: false,
  previewError: '',
  running: false,
  error: '',
  loading: true,
  loadError: '',
  /** Per-path reveal failure, shown next to the item. */
  revealErrors: {} as Record<string, string>,
})

const msg = (e: unknown) => (e instanceof Error ? e.message : String(e))
let unsub: (() => void) | undefined

/** Subscribes to ai:done once. */
export function initAIEvents(): void {
  if (unsub) return
  unsub = on('ai:done', (d: { report?: AIReportDTO | null; error?: string }) => {
    ai.running = false
    if (d?.error) {
      ai.error = d.error
      return
    }
    ai.error = ''
    if (d?.report) ai.report = d.report
  })
}

export async function loadAI(): Promise<void> {
  ai.loading = true
  ai.loadError = ''
  try {
    ai.profiles = (await ListAIProfiles()) ?? []
    if (!ai.profiles.some((p) => p.id === ai.profileId)) ai.profileId = ai.profiles[0]?.id ?? ''
    const rep = await GetAIReport().catch(() => null)
    if (rep && !ai.report) ai.report = rep
  } catch (e) {
    ai.loadError = `${strings.ai.loadError} ${msg(e)}`
  } finally {
    ai.loading = false
  }
}

export function selectProfile(id: string): void {
  ai.profileId = id
  ai.preview = null
  ai.previewError = ''
}

export async function loadPreview(): Promise<void> {
  ai.previewLoading = true
  ai.previewError = ''
  try {
    ai.preview = await PreviewAIPayload(ai.profileId)
  } catch (e) {
    ai.preview = null
    ai.previewError = `${strings.ai.previewError} ${msg(e)}`
  } finally {
    ai.previewLoading = false
  }
}

export async function runAnalysis(force: boolean): Promise<void> {
  ai.error = ''
  ai.running = true
  try {
    await RunAI(ai.profileId, force)
  } catch (e) {
    ai.running = false
    ai.error = msg(e)
  }
}

export async function cancelAnalysis(): Promise<void> {
  try {
    await CancelAI()
  } catch {
    /* ai:done reports the outcome */
  }
}

/** Opens Results on the parent folder of a path cited by the report. */
export async function revealItem(path: string): Promise<void> {
  delete ai.revealErrors[path]
  try {
    const loc = await LocatePath(path)
    await revealPath(loc.volume, loc.chain)
    route.volume = loc.volume
    route.current = 'results'
  } catch (e) {
    ai.revealErrors[path] = msg(e) || strings.ai.revealError
  }
}

/** Re-reads the report so item sizes/found flags follow the last scan (e.g. after a cleaning). */
export async function reloadReport(): Promise<void> {
  try {
    const rep = await GetAIReport()
    if (rep) ai.report = rep
  } catch {
    /* keep the report we have */
  }
}
