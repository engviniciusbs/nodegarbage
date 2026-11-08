<script lang="ts">
  import VerdictBadge from '../ui/VerdictBadge.svelte'
  import type { AIItemDTO } from '$lib/api'
  import { disabledReason, itemBytes, leafName, stricterVerdict } from '$lib/aireport'
  import { formatBytes } from '$lib/format'
  import { ai, revealItem } from '$lib/stores/ai.svelte'
  import { selection, toggle } from '$lib/stores/selection.svelte'
  import { strings, type Verdict } from '$lib/strings'

  let { item }: { item: AIItemDTO } = $props()

  const blocked = $derived(disabledReason(item))
  const verdict = $derived((item.verdict in strings.verdict ? item.verdict : 'unknown') as Verdict)
  // The selection is keyed by the scanned node path (the AI may cite it with a different case).
  const selected = $derived(!!item.node && selection.paths.has(item.node.path))
  const stricter = $derived(stricterVerdict(item) as Verdict | '')
  const reasonId = $derived(`ai-why-${encodeURIComponent(item.path)}`)
</script>

<li
  class="grid grid-cols-[20px_minmax(0,1fr)_auto] gap-x-3 gap-y-1 rounded-md border px-3 py-2.5 {selected
    ? 'border-accent bg-accent-tint'
    : 'border-stroke bg-card'}"
>
  <input
    type="checkbox"
    class="m-0 mt-0.5 size-4"
    aria-label={strings.ai.selectItem(item.path)}
    aria-describedby={blocked ? reasonId : undefined}
    title={blocked || undefined}
    disabled={blocked !== ''}
    checked={selected}
    onchange={() => item.node && toggle(item.node)}
  />
  <div class="flex min-w-0 flex-col gap-1">
    <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
      <span class="truncate text-[13px] font-semibold text-primary">{item.node?.name || leafName(item.path)}</span>
      <VerdictBadge {verdict} source="ai" compact />
      {#if stricter}
        <VerdictBadge
          verdict={stricter}
          suffix={item.node?.source === 'protected' ? strings.source.protected : strings.source.rule}
          label={strings.ai.effective}
        />
      {/if}
      <span class="text-xs text-muted tabular-nums">{strings.ai.confidence(Math.round(item.confidence * 100))}</span>
    </div>
    <button
      type="button"
      class="min-w-0 cursor-pointer truncate rounded-sm text-left font-mono text-xs text-accent hover:underline"
      title={item.path}
      aria-label={strings.ai.reveal(item.path)}
      onclick={() => revealItem(item.path)}>{item.path}</button
    >
    <!-- AI text is untrusted: plain text only, never rendered as HTML. -->
    <p class="m-0 text-[13px] text-primary">{item.reason}</p>
    {#if blocked}
      <p id={reasonId} class="m-0 text-xs text-muted">{blocked}</p>
    {/if}
    {#if ai.revealErrors[item.path]}
      <p role="alert" class="m-0 text-xs text-warn-text">{ai.revealErrors[item.path]}</p>
    {/if}
  </div>
  <span class="text-right text-[13px] font-semibold text-primary tabular-nums">
    {item.found ? formatBytes(itemBytes(item)) : '—'}
  </span>
</li>
