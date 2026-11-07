import { GetChildren } from '$lib/api'
import type { NodeDTO } from '$lib/api'
import { strings } from '$lib/strings'
import { clear } from './selection.svelte'

export const ROOT_ID = -1

export const results = $state({
  volume: '',
  currentId: ROOT_ID,
  path: [] as NodeDTO[],
  children: [] as NodeDTO[],
  categoryFilter: null as string | null,
  /** Row highlighted by pointer hover on the treemap; never moves DOM focus. */
  hoverId: null as number | null,
  /** One-shot request (e.g. treemap leaf click): the mounted TreeList scrolls to and focuses this row. */
  focusRequest: null as number | null,
  loading: false,
  error: '',
})

/** One-shot hint for the next TreeList mount: which row gets DOM focus (null = first). */
export const nav = { pendingFocus: undefined as { id: number | null } | undefined }

let seq = 0

async function fetchChildren(id: number): Promise<NodeDTO[] | null> {
  const mine = ++seq
  results.loading = true
  results.error = ''
  try {
    const kids = (await GetChildren(results.volume, id)) ?? []
    return mine === seq ? kids : null
  } catch (e) {
    if (mine === seq) results.error = `${strings.results.loadError} ${e instanceof Error ? e.message : String(e)}`
    return null
  } finally {
    if (mine === seq) results.loading = false
  }
}

/** Resets to the root of `volume` and loads it. Selection is cleared. */
export async function loadVolume(volume: string): Promise<void> {
  results.volume = volume
  results.path = []
  results.currentId = ROOT_ID
  results.children = []
  results.categoryFilter = null
  clear()
  if (!volume) return
  const kids = await fetchChildren(ROOT_ID)
  if (kids) results.children = kids
}

/** Drills into a child of the current folder. Never touches the selection. */
export async function openNode(id: number): Promise<void> {
  const node = results.children.find((c) => c.id === id)
  if (!node) return
  const kids = await fetchChildren(id)
  if (!kids) return
  nav.pendingFocus = { id: null }
  results.hoverId = null
  results.path = [...results.path, node]
  results.currentId = id
  results.children = kids
}

/** Truncates the breadcrumb to `index` entries (0 = root) and reloads. */
export async function goUp(index: number): Promise<void> {
  const i = Math.max(0, Math.min(index, results.path.length))
  const path = results.path.slice(0, i)
  const id = i === 0 ? ROOT_ID : path[i - 1].id
  const kids = await fetchChildren(id)
  if (!kids) return
  nav.pendingFocus = { id: results.path[i]?.id ?? null }
  results.hoverId = null
  results.path = path
  results.currentId = id
  results.children = kids
}

export async function refresh(): Promise<void> {
  const kids = await fetchChildren(results.currentId)
  if (kids) results.children = kids
}

/**
 * Opens the parent folder of a located item and asks the list to focus it.
 * `chain` runs from the first folder under the root down to the target.
 */
export async function revealPath(volume: string, chain: NodeDTO[]): Promise<void> {
  if (chain.length === 0) return
  const changed = results.volume !== volume
  results.volume = volume
  const parent = chain.slice(0, -1)
  const target = chain[chain.length - 1]
  const kids = await fetchChildren(parent.length ? parent[parent.length - 1].id : ROOT_ID)
  if (!kids) return
  if (changed) clear()
  results.hoverId = null
  results.categoryFilter = null
  results.path = parent
  results.currentId = parent.length ? parent[parent.length - 1].id : ROOT_ID
  results.children = kids
  results.focusRequest = target.id
}

export function toggleCategory(category: string): void {
  results.categoryFilter = results.categoryFilter === category ? null : category
}
