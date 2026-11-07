<script lang="ts">
  import Icon from '../ui/Icon.svelte'
  import VerdictBadge from '../ui/VerdictBadge.svelte'
  import type { NodeDTO } from '$lib/api'
  import { formatBytes, formatCount } from '$lib/format'
  import { blockedReason } from '$lib/stores/selection.svelte'
  import { ROW_HEIGHT } from '$lib/virtual'
  import { strings, type Verdict } from '$lib/strings'

  let {
    node,
    index,
    total,
    focused,
    selected,
    ontoggle,
    onfocus,
    onopen,
  }: {
    node: NodeDTO
    index: number
    total: number
    focused: boolean
    selected: boolean
    ontoggle: () => void
    onfocus: () => void
    onopen: () => void
  } = $props()

  const blocked = $derived(blockedReason(node))
  const verdict = $derived((node.verdict in strings.verdict ? node.verdict : 'unknown') as Verdict)
</script>

<!-- Keyboard handling lives on the parent tree (roving focus); the row only reacts to the pointer. -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  role="treeitem"
  aria-level="1"
  aria-posinset={index + 1}
  aria-setsize={total}
  aria-selected={focused}
  tabindex={focused ? 0 : -1}
  data-index={index}
  style="position:absolute;left:0;right:0;top:{index * ROW_HEIGHT}px;height:{ROW_HEIGHT}px"
  class="flex items-center gap-3 rounded-md px-2 text-sm text-primary outline-none hover:bg-black/5 focus-visible:ring-2 focus-visible:ring-accent {focused
    ? 'bg-accent/10'
    : ''}"
  onclick={onfocus}
  ondblclick={onopen}
>
  <input
    type="checkbox"
    tabindex="-1"
    class="size-4 shrink-0 accent-[var(--accent)]"
    aria-label={strings.results.select(node.name)}
    title={blocked ? strings.results.blocked[blocked] : undefined}
    disabled={blocked !== ''}
    checked={selected}
    onclick={(e) => e.stopPropagation()}
    onchange={ontoggle}
  />
  <span class="flex min-w-0 flex-1 items-center gap-2">
    <span class="truncate font-medium">{node.name}</span>
    {#if node.link}<Icon name="link" size={14} /><span class="sr-only">{strings.results.link}</span>{/if}
    {#if node.cloud}<Icon name="cloud" size={14} /><span class="sr-only">{strings.results.cloud}</span>{/if}
    {#if node.denied}<Icon name="denied" size={14} /><span class="sr-only">{strings.results.denied}</span>{/if}
    {#if node.inactive}
      <span class="shrink-0 rounded-sm bg-black/5 px-1.5 py-0.5 text-xs text-muted">{strings.results.inactive}</span>
    {/if}
  </span>
  <span class="hidden w-20 shrink-0 text-right text-xs text-muted tabular-nums sm:block">
    {strings.results.files(formatCount(node.files))}
  </span>
  <span class="w-24 shrink-0 text-right tabular-nums">{formatBytes(node.size)}</span>
  <span class="w-40 shrink-0"><VerdictBadge {verdict} source={node.source} /></span>
</div>
