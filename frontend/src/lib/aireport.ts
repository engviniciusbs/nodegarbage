import type { AIItemDTO, NodeDTO } from '$lib/api'
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

const RANK: Record<string, number> = { safe: 0, caution: 1, keep: 2 }

/**
 * The effective verdict (rules merged with the AI) when it is stricter than
 * the AI's own, e.g. AI "safe" on a folder a rule marks "caution"; else ''.
 */
export function stricterVerdict(it: AIItemDTO): string {
  const eff = it.found && it.node ? it.node.verdict : ''
  if (!(eff in RANK)) return ''
  return RANK[eff] > (RANK[it.verdict] ?? RANK.caution) ? eff : ''
}

/** What "Selecionar todos" may add: selectable items whose effective verdict is not stricter than the AI's. */
export function bulkSelectable(items: AIItemDTO[]): NodeDTO[] {
  return items.filter((it) => disabledReason(it) === '' && !stricterVerdict(it)).map((it) => it.node as NodeDTO)
}

/** Why the item's checkbox is disabled, or '' when it can be selected. */
export function disabledReason(it: AIItemDTO): string {
  if (!it.found || !it.node) return strings.ai.missing[it.missing] ?? strings.ai.notFound
  const b = blockedReason(it.node)
  if (b) return strings.results.blocked[b]
  return it.selectable ? '' : strings.ai.notSelectable
}

export { leafName } from './format'
