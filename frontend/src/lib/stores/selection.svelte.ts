import { SvelteMap } from 'svelte/reactivity'
import type { NodeDTO } from '$lib/api'

export const selection = $state({ paths: new SvelteMap<string, NodeDTO>() })

/** Why a node can never be selected for cleaning, or '' when it can. */
export function blockedReason(n: NodeDTO): 'keep' | 'manual' | 'link' | '' {
  if (n.verdict === 'keep') return 'keep'
  if (n.action === 'manual') return 'manual'
  if (n.link) return 'link'
  return ''
}

/** Nothing is ever pre-selected; protected/manual/link nodes are ignored. */
export function toggle(n: NodeDTO): void {
  if (blockedReason(n)) return
  if (selection.paths.has(n.path)) selection.paths.delete(n.path)
  else selection.paths.set(n.path, n)
}

/** Explicit "select all" click: adds every selectable node; never deselects. */
export function selectAll(nodes: NodeDTO[]): void {
  for (const n of nodes) if (!blockedReason(n) && !selection.paths.has(n.path)) selection.paths.set(n.path, n)
}

export function clear(): void {
  selection.paths.clear()
}

export function selectedBytes(): number {
  let total = 0
  for (const n of selection.paths.values()) total += n.size
  return total
}
