import type { AIItemDTO } from '$lib/api'
import { blockedReason } from '$lib/stores/selection.svelte'
import { strings } from '$lib/strings'

export type AIGroupId = 'safe' | 'caution' | 'keep'
export const GROUP_ORDER: AIGroupId[] = ['safe', 'caution', 'keep']

/** The AI verdict decides the group; anything unexpected is "Avalie antes" (the cautious middle). */
export function groupOf(verdict: string): AIGroupId {
  return verdict === 'safe' ? 'safe' : verdict === 'keep' ? 'keep' : 'caution'
}

export function groupItems(items: AIItemDTO[]): Record<AIGroupId, AIItemDTO[]> {
  const out: Record<AIGroupId, AIItemDTO[]> = { safe: [], caution: [], keep: [] }
  for (const it of items) out[groupOf(it.verdict)].push(it)
  return out
}

/** Size from the last scan (0 when the path is not in it). */
export const itemBytes = (it: AIItemDTO) => (it.found && it.node ? it.node.size : 0)

export const sumBytes = (items: AIItemDTO[]) => items.reduce((t, it) => t + itemBytes(it), 0)

/** Why the item's checkbox is disabled, or '' when it can be selected. */
export function disabledReason(it: AIItemDTO): string {
  if (!it.found || !it.node) return strings.ai.notFound
  const b = blockedReason(it.node)
  if (b) return strings.results.blocked[b]
  return it.selectable ? '' : strings.ai.notSelectable
}

/** Last path component, for the row title. */
export function leafName(path: string): string {
  const parts = path.split(/[\\/]/).filter(Boolean)
  return parts[parts.length - 1] ?? path
}
