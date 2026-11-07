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
    hovered = false,
    ontoggle,
    onfocus,
    onopen,
  }: {
    node: NodeDTO
    index: number
    total: number
    focused: boolean
    selected: boolean
    hovered?: boolean
    ontoggle: () => void
    onfocus: () => void
    onopen: () => void
  } = $props()

  const blocked = $derived(blockedReason(node))
  const verdict = $derived((node.verdict in strings.verdict ? node.verdict : 'unknown') as Verdict)
  // Focused wins over selected, which wins over the treemap hover highlight.
  const tone = $derived(
    focused
      ? 'bg-accent-focus shadow-[inset_2px_0_0_var(--accent)]'
      : selected
        ? 'bg-accent-tint'
        : hovered
          ? 'bg-hover'
          : 'hover:bg-hover-soft',
  )
</script>

<!-- Keyboard handling lives on the parent tree (roving focus); the row only reacts to the pointer. -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
  role="treeitem"
  aria-level="1"
  aria-posinset={index + 1}
  aria-setsize={total}
  aria-selected={selected}
  tabindex={focused ? 0 : -1}
  data-index={index}
  style="position:absolute;left:0;right:0;top:{index * ROW_HEIGHT}px;height:{ROW_HEIGHT}px"
  class="list-grid border-b border-stroke-soft text-[13px] text-primary outline-none focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-accent {tone}"
  onclick={onfocus}
  ondblclick={onopen}
>
  <input
    type="checkbox"
    tabindex="-1"
    class="m-0 size-4"
    aria-label={strings.results.select(node.name)}
    title={blocked ? strings.results.blocked[blocked] : undefined}
    disabled={blocked !== ''}
    checked={selected}
    onclick={(e) => e.stopPropagation()}
    onchange={ontoggle}
  />
  <span class="flex min-w-0 items-center gap-2">
    <span class="shrink-0 text-muted"><Icon name={node.hasChildren ? 'folder' : 'file'} size={16} /></span>
    <span class="truncate">{node.name}</span>
    {#if node.link}<span class="shrink-0 text-muted"><Icon name="link" size={14} /></span><span class="sr-only">{strings.results.link}</span>{/if}
    {#if node.cloud}<span class="shrink-0 text-muted"><Icon name="cloud" size={14} /></span><span class="sr-only">{strings.results.cloud}</span>{/if}
    {#if node.denied}<span class="shrink-0 text-muted"><Icon name="denied" size={14} /></span><span class="sr-only">{strings.results.denied}</span>{/if}
    {#if node.inactive}
      <span class="shrink-0 rounded-full border border-inactive-border px-1.5 py-px text-[11px] whitespace-nowrap text-warn-text">{strings.results.inactive}</span>
    {/if}
  </span>
  <span class="text-right font-semibold tabular-nums">{formatBytes(node.size)}</span>
  <span class="col-files text-right text-muted tabular-nums">{formatCount(node.files)}</span>
  <span class="flex min-w-0"><VerdictBadge {verdict} source={node.source} compact /></span>
</div>
