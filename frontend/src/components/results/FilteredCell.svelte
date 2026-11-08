<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import NodeFlags from './NodeFlags.svelte'
  import type { NodeDTO } from '$lib/api'
  import { leafName } from '$lib/format'
  import { strings } from '$lib/strings'

  let { node }: { node: NodeDTO } = $props()

  // Line 1 names the project (the parent folder); an item not called node_modules says which one it is.
  const parent = $derived(leafName(node.location) || node.location)
  const title = $derived(node.name.toLowerCase() === 'node_modules' ? parent : strings.results.itemIn(node.name, parent))
</script>

<!-- Filtered view "Local" cell: two lines inside the fixed 40px row. -->
<span class="flex min-w-0 items-center gap-2">
  <span class="shrink-0 text-muted"><Icon name="folder" size={16} /></span>
  <span class="flex min-w-0 flex-1 flex-col">
    <!-- One visible line: a pill that does not fit wraps onto a clipped second line instead of overlapping. -->
    <span class="flex h-[18px] min-w-0 flex-wrap items-center gap-x-2 overflow-hidden leading-[18px]">
      <span class="max-w-full truncate text-[13px] font-semibold">{title}</span>
      <NodeFlags {node} />
    </span>
    <!-- rtl box + ltr text: the ellipsis eats the START of the path, so its end stays visible. -->
    <span dir="rtl" title={node.location} class="truncate text-left font-mono text-[11px] leading-[15px] text-muted"
      ><bdi dir="ltr">{node.location}</bdi></span
    >
  </span>
</span>
