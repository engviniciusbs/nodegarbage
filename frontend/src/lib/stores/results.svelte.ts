import { GetChildren, ListFiltered, LocatePath } from '$lib/api'
import type { FilterDTO, FilteredDTO, NodeDTO } from '$lib/api'
import { strings } from '$lib/strings'
import { clear } from './selection.svelte'

export const ROOT_ID = -1
/** Rule behind the fixed "node_modules" shortcut chip. */
export const NODE_MODULES_RULE = 'node.node_modules'

/** A volume-wide filter (exactly one of category / ruleId) and its display label. */
export type ActiveFilter = { category: string; ruleId: string; label: string }

export const results = $state({
  volume: '',
  currentId: ROOT_ID,
  path: [] as NodeDTO[],
  children: [] as NodeDTO[],
  /** Volume-wide filter; while set, the list shows `filtered` instead of the current folder. */
  filter: null as ActiveFilter | null,
  filtered: null as FilteredDTO | null,
  filterLoading: false,
  filterError: '',
  /** Non-blocking problem in the filtered view (e.g. an item could not be opened); the list stays. */
  filterNotice: '',
  /** node_modules matches of the volume (null until loaded, or on error). */
  shortcut: null as { count: number; totalBytes: number } | null,
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
  resetFilter()
  shortcutSeq++
  results.shortcut = null
  clear()
  if (!volume) return
  void loadShortcut()
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
  // Nothing loaded (e.g. a cleaning started from the AI screen before Results was opened).
  if (!results.volume) return
  if (results.filter) void fetchFiltered()
  void loadShortcut()
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
  resetFilter()
  results.path = parent
  results.currentId = parent.length ? parent[parent.length - 1].id : ROOT_ID
  results.children = kids
  results.focusRequest = target.id
}

// ---- Volume-wide filtered view -------------------------------------------

let filterSeq = 0
let shortcutSeq = 0
const errText = (e: unknown) => (e instanceof Error ? e.message : String(e))

function resetFilter() {
  filterSeq++
  results.filter = null
  results.filtered = null
  results.filterLoading = false
  results.filterError = ''
  results.filterNotice = ''
}

async function fetchFiltered(): Promise<void> {
  const f = results.filter
  if (!f) return
  const mine = ++filterSeq
  results.filterLoading = true
  results.filterError = ''
  try {
    const dto: FilterDTO = { category: f.category, ruleId: f.ruleId }
    const res = await ListFiltered(results.volume, dto)
    if (mine === filterSeq) results.filtered = res
  } catch (e) {
    if (mine === filterSeq) results.filterError = `${strings.results.loadError} ${errText(e)}`
  } finally {
    if (mine === filterSeq) results.filterLoading = false
  }
}

/** Shows the volume-wide matches of a category or rule; the same filter again turns it off. */
export function toggleFilter(f: ActiveFilter): Promise<void> {
  const cur = results.filter
  if (cur && cur.category === f.category && cur.ruleId === f.ruleId) {
    clearFilter()
    return Promise.resolve()
  }
  results.filter = { ...f }
  results.filtered = null
  results.filterNotice = ''
  results.hoverId = null
  return fetchFiltered()
}

/** Leaves the filtered view: the folder navigation underneath was never changed. */
export function clearFilter(): void {
  if (!results.filter) return
  resetFilter()
  results.hoverId = null
  nav.pendingFocus = { id: null }
}

export function retryFilter(): Promise<void> {
  return fetchFiltered()
}

/** Counts the node_modules of the volume for the shortcut chip. Silent on failure. */
async function loadShortcut(): Promise<void> {
  const volume = results.volume
  if (!volume) return
  const mine = ++shortcutSeq
  try {
    const res = await ListFiltered(volume, { category: '', ruleId: NODE_MODULES_RULE })
    if (mine === shortcutSeq) results.shortcut = { count: res.count, totalBytes: res.totalBytes }
  } catch {
    if (mine === shortcutSeq) results.shortcut = null
  }
}

/**
 * Leaves the filtered view for the folder of a filtered item: its parent with
 * the item focused, or (inside) the item's own folder.
 */
export async function openFiltered(n: NodeDTO, inside = false): Promise<void> {
  results.filterNotice = ''
  try {
    const loc = await LocatePath(n.path)
    await revealPath(loc.volume, loc.chain)
    if (inside && results.filter === null) await openNode(loc.chain[loc.chain.length - 1].id)
  } catch (e) {
    results.filterNotice = `${strings.results.locateError} ${errText(e)}`
  }
}
