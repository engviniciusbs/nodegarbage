import type { NodeDTO } from '$lib/api'

export function node(over: Partial<NodeDTO> = {}): NodeDTO {
  const id = over.id ?? 1
  return {
    id,
    name: `item-${id}`,
    path: `C:\\data\\item-${id}`,
    size: 1024 * 1024,
    files: 10,
    latestMod: 1_700_000_000,
    hasChildren: true,
    link: false,
    cloud: false,
    denied: false,
    inactive: false,
    verdict: 'safe',
    category: 'dev-cache',
    source: 'rule',
    ruleId: 'r1',
    action: 'delete',
    explain: 'Cache seguro de remover.',
    regenerable: '',
    command: '',
    ...over,
  } as NodeDTO
}
