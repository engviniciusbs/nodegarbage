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
  loading: false,
  error: '',
})

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
  results.path = path
  results.currentId = id
  results.children = kids
}

export async function refresh(): Promise<void> {
  const kids = await fetchChildren(results.currentId)
  if (kids) results.children = kids
}

export function toggleCategory(category: string): void {
  results.categoryFilter = results.categoryFilter === category ? null : category
}
