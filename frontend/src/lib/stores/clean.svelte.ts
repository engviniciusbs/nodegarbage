import { SvelteSet } from 'svelte/reactivity'
import { PlanClean, ExecuteClean, CancelClean, on } from '$lib/api'
import type { PlanDTO, PlanItemDTO } from '$lib/api'
import { strings } from '$lib/strings'
import { clear as clearSelection } from './selection.svelte'
import { refresh } from './results.svelte'

export type CleanMode = 'recycle' | 'permanent'
export type CleanPhase = 'planning' | 'review' | 'executing' | 'done'

export interface ItemResult {
  path: string
  status: string
  reason: string
  freed: number
  skippedCount?: number
}

export const clean = $state({
  phase: 'planning' as CleanPhase,
  mode: 'recycle' as CleanMode,
  plan: null as PlanDTO | null,
  /** Why the plan could not be built, or why the last execution request failed. */
  error: '',
  /** The failed ExecuteClean call may have consumed the token: a new plan is required. */
  needsNewPlan: false,
  paths: [] as string[],
  progress: { done: 0, total: 0, current: '' },
  canceling: false,
  summary: null as { freed: number; results: ItemResult[]; warning: string } | null,
  permanentAck: false,
  /** Incremented after every finished cleaning so screens can refresh derived data. */
  doneCount: 0,
})

/** Caution/unknown items the user explicitly accepted ("Entendo o risco"). */
export const riskAck = new SvelteSet<string>()

let seq = 0
let busy = false
let unsubs: (() => void)[] = []

const msg = (e: unknown) => (typeof e === 'string' ? e : e instanceof Error ? e.message : String(e))

export const needsRisk = (it: PlanItemDTO) => it.verdict !== 'safe'

export function canConfirm(): boolean {
  const p = clean.plan
  if (clean.phase !== 'review' || !p || clean.needsNewPlan || clean.error) return false
  if (p.items.length === 0) return false
  if (clean.mode === 'permanent' && !clean.permanentAck) return false
  return p.items.every((it) => !needsRisk(it) || riskAck.has(it.path))
}

function unsubscribe() {
  unsubs.forEach((u) => u())
  unsubs = []
}

async function plan(): Promise<void> {
  const mine = ++seq
  clean.phase = 'planning'
  clean.plan = null
  clean.error = ''
  clean.needsNewPlan = false
  clean.permanentAck = false
  riskAck.clear()
  try {
    const p = await PlanClean(clean.paths, clean.mode)
    if (mine !== seq) return
    clean.plan = p
  } catch (e) {
    if (mine !== seq) return
    clean.error = `${strings.clean.planError} ${msg(e)}`
  }
  clean.phase = 'review'
}

/** Opens a fresh session for `paths` (a snapshot of the selection) in recycle mode. */
export function startClean(paths: string[]): Promise<void> {
  // Never drop the listeners of a running cleaning.
  if (clean.phase === 'executing') return Promise.resolve()
  unsubscribe()
  busy = false
  clean.mode = 'recycle'
  clean.paths = [...paths]
  clean.summary = null
  clean.canceling = false
  clean.progress = { done: 0, total: 0, current: '' }
  return plan()
}

/** Changing the mode always builds a new plan (new single-use token). */
export function setMode(mode: CleanMode): Promise<void> | void {
  if (clean.phase === 'executing' || clean.phase === 'done' || mode === clean.mode) return
  clean.mode = mode
  return plan()
}

export function replan(): Promise<void> | void {
  if (clean.phase === 'executing' || clean.phase === 'done') return
  return plan()
}

/** Invalidates in-flight planning; never interrupts a running cleaning. */
export function resetClean(): void {
  if (clean.phase === 'executing') return
  seq++
  unsubscribe()
  clean.plan = null
}

function finish(d: { freed: number; results: ItemResult[]; warning?: string }) {
  unsubscribe()
  busy = false
  clean.summary = { freed: d?.freed ?? 0, results: d?.results ?? [], warning: d?.warning ?? '' }
  clean.phase = 'done'
  clean.doneCount++
  clearSelection()
  void refresh()
}

/** Redeems the plan token exactly once. The only caller of ExecuteClean. */
export async function execute(): Promise<void> {
  const p = clean.plan
  if (busy || !p || !canConfirm()) return
  busy = true
  clean.phase = 'executing'
  clean.progress = { done: 0, total: p.items.length, current: '' }
  unsubscribe()
  unsubs = [
    on('clean:progress', (d: { index: number; total: number; result: ItemResult }) => {
      clean.progress = { done: d.index + 1, total: d.total, current: d.result?.path ?? '' }
    }),
    on('clean:done', finish),
  ]
  try {
    await ExecuteClean(p.id, p.token)
  } catch (e) {
    unsubscribe()
    busy = false
    const text = msg(e)
    clean.phase = 'review'
    clean.needsNewPlan = true
    clean.error = /expired|expirad|expirou|token/i.test(text)
      ? strings.clean.expired
      : `${strings.clean.executeError} ${text}`
  }
}

export function cancelRun(): void {
  if (clean.phase !== 'executing' || clean.canceling) return
  clean.canceling = true
  void CancelClean()
}
